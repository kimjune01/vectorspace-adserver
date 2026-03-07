/**
 * CloudX Publisher SDK
 *
 * Usage:
 *   const cloudx = new CloudX({ endpoint: "https://ads.cloudx.dev" });
 *   const ad = await cloudx.requestAd({ intent: "dog training near me", tau: 0.6 });
 *   if (ad) {
 *     renderAd(ad.winner);
 *   }
 *
 *   // Or from a chat conversation:
 *   const ad = await cloudx.requestAdFromChat(messages, 0.6);
 */

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface CloudXConfig {
  /** Ad server endpoint (no trailing slash) */
  endpoint: string;
}

export interface AdRequestOptions {
  /** The user's intent — what they're looking for */
  intent: string;
  /** Relevance threshold (distance² cutoff). Lower = stricter. Omit to allow all. */
  tau?: number;
  /** Publisher ID to attribute auctions to this publisher */
  publisherId?: string;
}

export interface AdBidder {
  id: string;
  rank: number;
  name: string;
  intent: string;
  bid_price: number;
  sigma: number;
  score: number;
  distance_sq: number;
  log_bid: number;
  click_url?: string;
}

export interface AdResponse {
  auction_id: number;
  intent: string;
  winner: AdBidder | null;
  runner_up: AdBidder | null;
  all_bidders: AdBidder[];
  payment: number;
  currency: string;
  bid_count: number;
  eligible_count: number;
}

const INTENT_PROMPT = `Given a conversation, decide whether the person could benefit from a professional service. If yes, write a single sentence describing that service — as if the provider were writing their own position statement. If the conversation is casual, off-topic, or doesn't suggest any professional need, respond with exactly "NONE".

Rules:
- Match the most obvious need. A health complaint needs a health provider, not a lawyer. A legal issue needs legal help, not a therapist.
- Write in third person as a service description: "[Role] providing/helping/specializing in [what they do]"
- Be specific to the situation but don't embellish beyond what's stated.
- Do NOT extract demographics or personal data about the user.
- If there is no clear professional need, respond with "NONE".

Respond with ONLY the one-sentence service description or "NONE", nothing else.`;

export interface CachedEmbedding {
  id: string;
  name: string;
  embedding: number[];
  bid_price: number;
  sigma: number;
  currency: string;
}

export interface LocalAuctionResult {
  winner: AdBidder;
  runnerUp: AdBidder | null;
  allBidders: AdBidder[];
  payment: number;
  currency: string;
}

export interface ProximityResult {
  id: string;
  distance: number;
}

const LOG_BASE = 5.0;

/** Client-side frequency capping — no user_id ever leaves the browser. */
export class FrequencyCapLocal {
  private caps = new Map<string, { count: number; windowStart: number }>();

  check(advertiserId: string, maxImpressions: number, windowSeconds: number): boolean {
    const entry = this.caps.get(advertiserId);
    if (!entry) return true;
    if (Date.now() - entry.windowStart > windowSeconds * 1000) return true;
    return entry.count < maxImpressions;
  }

  increment(advertiserId: string, windowSeconds: number): void {
    const entry = this.caps.get(advertiserId);
    const now = Date.now();
    if (!entry || now - entry.windowStart > windowSeconds * 1000) {
      this.caps.set(advertiserId, { count: 1, windowStart: now });
    } else {
      entry.count++;
    }
  }
}

export interface TEEAdResponse {
  auction_id: number;
  winner_id: string;
  payment: number;
  currency: string;
  bid_count: number;
}

export interface EncryptedEmbedding {
  aes_key_encrypted: string;
  encrypted_payload: string;
  nonce: string;
  hash_algorithm: string;
}

export class CloudX {
  private endpoint: string;
  private embeddingCache: CachedEmbedding[] = [];
  private embeddingEtag: string | null = null;
  readonly frequencyCap = new FrequencyCapLocal();

  // TEE attestation cache
  private teePubKey: CryptoKey | null = null;
  private teeAttestation: { public_key: string; attestation_cose_base64: string } | null = null;

  constructor(config: CloudXConfig) {
    this.endpoint = config.endpoint.replace(/\/+$/, "");
  }

  /**
   * Fetch advertiser embeddings from the server.
   * Uses ETag/If-None-Match for efficient caching — a 304 means no update needed.
   */
  async syncEmbeddings(): Promise<void> {
    const headers: Record<string, string> = {};
    if (this.embeddingEtag) {
      headers["If-None-Match"] = this.embeddingEtag;
    }

    const resp = await fetch(`${this.endpoint}/embeddings`, { headers });
    if (resp.status === 304) return; // cache is fresh

    if (!resp.ok) {
      throw new Error(`Embeddings API error: ${resp.status}`);
    }

    const data = await resp.json();
    this.embeddingCache = data.embeddings;
    this.embeddingEtag = resp.headers.get("ETag");
  }

  /**
   * Embed arbitrary text via the server's embedding sidecar.
   * Returns the embedding vector.
   */
  async embed(text: string): Promise<number[]> {
    const resp = await fetch(`${this.endpoint}/embed`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text }),
    });

    if (!resp.ok) {
      const err = await resp.text().catch(() => "");
      throw new Error(err || `Embed API error: ${resp.status}`);
    }

    const data = await resp.json();
    return data.embedding;
  }

  /**
   * Compute squared Euclidean distance from queryEmbedding to each cached
   * advertiser embedding. Returns results sorted ascending (closest first).
   * Pure local math — no network call.
   */
  proximity(queryEmbedding: number[]): ProximityResult[] {
    return this.embeddingCache
      .map((entry) => ({
        id: entry.id,
        distance: squaredEuclidean(queryEmbedding, entry.embedding),
      }))
      .sort((a, b) => a.distance - b.distance);
  }

  /**
   * Extract a service-description intent from a chat conversation.
   * Calls the /chat endpoint with the INTENT_PROMPT system message.
   */
  async extractIntent(messages: ChatMessage[]): Promise<string> {
    const resp = await fetch(`${this.endpoint}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messages, system: INTENT_PROMPT }),
    });
    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Chat API error: ${resp.status}`);
    }
    const data = await resp.json();
    return data.content;
  }

  /**
   * Extract intent from a conversation and request a matching ad in one call.
   * Returns null if no ad passed the relevance gate.
   */
  async requestAdFromChat(
    messages: ChatMessage[],
    tau?: number
  ): Promise<AdResponse | null> {
    const intent = await this.extractIntent(messages);
    if (intent === "NONE") return null;
    return this.requestAd({ intent, tau });
  }

  /**
   * Request an ad for the given intent.
   * Returns null if no ad passed the relevance gate.
   */
  async requestAd(options: AdRequestOptions): Promise<AdResponse | null> {
    const body: Record<string, unknown> = { intent: options.intent };
    if (options.tau != null && options.tau > 0) {
      body.tau = options.tau;
    }
    if (options.publisherId) {
      body.publisher_id = options.publisherId;
    }

    const resp = await fetch(`${this.endpoint}/ad-request`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      if (resp.status === 500 && text.includes("no bidders passed")) {
        return null; // no ads met the relevance threshold — expected
      }
      throw new Error(text || `CloudX error ${resp.status}`);
    }

    return resp.json();
  }

  // ── Local Auction (private flow) ────────────────────────────────

  /**
   * Run a full auction locally using cached embeddings + bid data.
   * Replicates the server's scoring (log_5(price) - dist²/σ²) and VCG payment.
   * Returns null if no bidders pass the relevance gate.
   */
  runLocalAuction(
    queryEmbedding: number[],
    tau?: number
  ): LocalAuctionResult | null {
    if (this.embeddingCache.length === 0) return null;

    // Score all bidders
    const scored: {
      entry: CachedEmbedding;
      distSq: number;
      score: number;
      logBid: number;
    }[] = [];

    for (const entry of this.embeddingCache) {
      if (entry.bid_price <= 0) continue;
      const distSq = squaredEuclidean(queryEmbedding, entry.embedding);
      if (tau != null && tau > 0 && distSq > tau) continue;
      const logBid = Math.log(entry.bid_price) / Math.log(LOG_BASE);
      const score =
        entry.sigma > 0 ? logBid - distSq / (entry.sigma * entry.sigma) : logBid;
      scored.push({ entry, distSq, score, logBid });
    }

    if (scored.length === 0) return null;

    // Rank by score descending
    scored.sort((a, b) => b.score - a.score);

    // VCG payment
    const winner = scored[0];
    const runnerUp = scored.length > 1 ? scored[1] : null;
    let payment: number;

    if (runnerUp) {
      const sigmaW = winner.entry.sigma;
      const sigmaR = runnerUp.entry.sigma;
      if (sigmaW > 0 && sigmaR > 0) {
        payment =
          runnerUp.entry.bid_price *
          Math.pow(
            LOG_BASE,
            winner.distSq / (sigmaW * sigmaW) -
              runnerUp.distSq / (sigmaR * sigmaR)
          );
      } else {
        payment = runnerUp.entry.bid_price;
      }
    } else {
      payment = winner.entry.bid_price;
    }

    // Cap at winner's bid (individual rationality)
    if (payment > winner.entry.bid_price) {
      payment = winner.entry.bid_price;
    }

    const toBidder = (
      s: (typeof scored)[number],
      rank: number
    ): AdBidder => ({
      id: s.entry.id,
      rank,
      name: s.entry.name,
      intent: "",
      bid_price: s.entry.bid_price,
      sigma: s.entry.sigma,
      score: s.score,
      distance_sq: s.distSq,
      log_bid: s.logBid,
    });

    return {
      winner: toBidder(winner, 1),
      runnerUp: runnerUp ? toBidder(runnerUp, 2) : null,
      allBidders: scored.map((s, i) => toBidder(s, i + 1)),
      payment,
      currency: winner.entry.currency,
    };
  }

  /**
   * Privacy-preserving ad request: runs the auction locally, then only
   * tells the exchange who won and how much to charge. No intent text,
   * no user ID ever leaves the browser.
   */
  async requestAdPrivate(
    options: AdRequestOptions
  ): Promise<AdResponse | null> {
    const queryEmbedding = await this.embed(options.intent);
    const local = this.runLocalAuction(queryEmbedding, options.tau);
    if (!local) return null;

    // Claim the win on the server (billing only)
    const resp = await fetch(`${this.endpoint}/ad-claim`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        winner_id: local.winner.id,
        payment: local.payment,
        publisher_id: options.publisherId ?? "",
      }),
    });

    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Ad-claim error ${resp.status}`);
    }

    const claim: { auction_id: number } = await resp.json();

    return {
      auction_id: claim.auction_id,
      intent: options.intent,
      winner: local.winner,
      runner_up: local.runnerUp,
      all_bidders: local.allBidders,
      payment: local.payment,
      currency: local.currency,
      bid_count: local.allBidders.length,
      eligible_count: local.allBidders.length,
    };
  }

  /**
   * Privacy-preserving chat → ad flow: extracts intent locally
   * (via publisher's chat endpoint), then runs the auction on-device.
   */
  async requestAdFromChatPrivate(
    messages: ChatMessage[],
    tau?: number,
    publisherId?: string
  ): Promise<AdResponse | null> {
    const intent = await this.extractIntent(messages);
    if (intent === "NONE") return null;
    return this.requestAdPrivate({ intent, tau, publisherId });
  }

  // ── TEE-Encrypted Auction (Phase 2) ──────────────────────────

  /**
   * Fetch the enclave's attested public key from the exchange.
   * Caches the key for subsequent encrypt calls.
   */
  async fetchAttestation(): Promise<void> {
    const resp = await fetch(`${this.endpoint}/tee/attestation`);
    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Attestation error: ${resp.status}`);
    }

    const data: { public_key: string; attestation_cose_base64: string } =
      await resp.json();
    this.teeAttestation = data;
    this.teePubKey = await importRSAPublicKey(data.public_key);
  }

  /**
   * Encrypt an embedding vector using the enclave's attested public key.
   * Uses hybrid RSA-OAEP(SHA-256) + AES-256-GCM via Web Crypto API.
   */
  async encryptEmbedding(
    embedding: number[],
    publicKey: CryptoKey
  ): Promise<EncryptedEmbedding> {
    // Generate random AES-256 key
    const aesKey = await crypto.subtle.generateKey(
      { name: "AES-GCM", length: 256 },
      true,
      ["encrypt"]
    );

    // Export AES key as raw bytes
    const aesKeyRaw = await crypto.subtle.exportKey("raw", aesKey);

    // Encrypt AES key with RSA-OAEP(SHA-256)
    const aesKeyEncrypted = await crypto.subtle.encrypt(
      { name: "RSA-OAEP" },
      publicKey,
      aesKeyRaw
    );

    // Encrypt embedding payload with AES-256-GCM
    const nonce = crypto.getRandomValues(new Uint8Array(12));
    const payload = new TextEncoder().encode(JSON.stringify(embedding));
    const encryptedPayload = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv: nonce },
      aesKey,
      payload
    );

    return {
      aes_key_encrypted: toBase64(aesKeyEncrypted),
      encrypted_payload: toBase64(encryptedPayload),
      nonce: toBase64(nonce.buffer),
      hash_algorithm: "SHA-256",
    };
  }

  /**
   * TEE-private ad request: embed intent locally, encrypt the embedding
   * with the enclave's attested key, send ciphertext to the exchange.
   * The exchange never sees the embedding — only the enclave decrypts it.
   */
  async requestAdTEE(
    options: AdRequestOptions
  ): Promise<TEEAdResponse | null> {
    // Ensure we have the enclave's public key
    if (!this.teePubKey) {
      await this.fetchAttestation();
    }

    // Embed the intent text via the publisher's sidecar
    const embedding = await this.embed(options.intent);

    // Encrypt the embedding with the enclave's public key
    const encrypted = await this.encryptEmbedding(embedding, this.teePubKey!);

    // Send encrypted embedding to the exchange
    const body: Record<string, unknown> = {
      encrypted_embedding: encrypted,
    };
    if (options.tau != null && options.tau > 0) {
      body.tau = options.tau;
    }
    if (options.publisherId) {
      body.publisher_id = options.publisherId;
    }

    const resp = await fetch(`${this.endpoint}/ad-request-private`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      if (resp.status === 500 && text.includes("no bidders passed")) {
        return null;
      }
      throw new Error(text || `TEE ad-request error ${resp.status}`);
    }

    return resp.json();
  }

  /**
   * TEE-private chat → ad flow: extract intent from chat, then run TEE auction.
   */
  async requestAdFromChatTEE(
    messages: ChatMessage[],
    tau?: number,
    publisherId?: string
  ): Promise<TEEAdResponse | null> {
    const intent = await this.extractIntent(messages);
    if (intent === "NONE") return null;
    return this.requestAdTEE({ intent, tau, publisherId });
  }

  // ── Private-flow event helpers ────────────────────────────────

  /** Report impression without sending user_id. */
  async reportImpressionPrivate(
    auctionId: number,
    advertiserId: string,
    publisherId?: string
  ): Promise<boolean> {
    return this.reportImpression(auctionId, advertiserId, undefined, publisherId);
  }

  /** Report click without sending user_id. */
  async reportClickPrivate(
    auctionId: number,
    advertiserId: string,
    publisherId?: string
  ): Promise<void> {
    return this.reportClick(auctionId, advertiserId, undefined, publisherId);
  }

  /** Report viewability without sending user_id. */
  async reportViewablePrivate(
    auctionId: number,
    advertiserId: string,
    publisherId?: string
  ): Promise<void> {
    return this.reportViewable(auctionId, advertiserId, undefined, publisherId);
  }

  // ── Event Tracking ──────────────────────────────────────────────

  /**
   * Report an impression event. Returns false if frequency-capped (429).
   */
  async reportImpression(
    auctionId: number,
    advertiserId: string,
    userId?: string,
    publisherId?: string
  ): Promise<boolean> {
    const body: Record<string, unknown> = {
      auction_id: auctionId,
      advertiser_id: advertiserId,
    };
    if (userId) body.user_id = userId;
    if (publisherId) body.publisher_id = publisherId;

    const resp = await fetch(`${this.endpoint}/event/impression`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (resp.status === 429) return false;
    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Impression error ${resp.status}`);
    }
    return true;
  }

  /**
   * Report a click event.
   */
  async reportClick(
    auctionId: number,
    advertiserId: string,
    userId?: string,
    publisherId?: string
  ): Promise<void> {
    const body: Record<string, unknown> = {
      auction_id: auctionId,
      advertiser_id: advertiserId,
    };
    if (userId) body.user_id = userId;
    if (publisherId) body.publisher_id = publisherId;

    const resp = await fetch(`${this.endpoint}/event/click`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Click error ${resp.status}`);
    }
  }

  /**
   * Report a viewability event.
   */
  async reportViewable(
    auctionId: number,
    advertiserId: string,
    userId?: string,
    publisherId?: string
  ): Promise<void> {
    const body: Record<string, unknown> = {
      auction_id: auctionId,
      advertiser_id: advertiserId,
    };
    if (userId) body.user_id = userId;
    if (publisherId) body.publisher_id = publisherId;

    const resp = await fetch(`${this.endpoint}/event/viewable`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Viewable error ${resp.status}`);
    }
  }

  /**
   * Observe an ad element for IAB viewability (50%+ visible for 1+ second).
   * Fires reportViewable automatically when the threshold is met.
   */
  observeViewability(
    element: HTMLElement,
    auctionId: number,
    advertiserId: string,
    userId?: string
  ): void {
    let timer: ReturnType<typeof setTimeout> | null = null;

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry.intersectionRatio >= 0.5) {
          if (!timer) {
            timer = setTimeout(() => {
              this.reportViewable(auctionId, advertiserId, userId);
              observer.disconnect();
            }, 1000);
          }
        } else {
          if (timer) {
            clearTimeout(timer);
            timer = null;
          }
        }
      },
      { threshold: [0, 0.5] }
    );

    observer.observe(element);
  }
}

/** Squared Euclidean distance: ||a - b||² */
function squaredEuclidean(a: number[], b: number[]): number {
  let sum = 0;
  for (let i = 0; i < a.length; i++) {
    const d = a[i] - b[i];
    sum += d * d;
  }
  return sum;
}

/** Import a PEM-encoded RSA public key into Web Crypto for RSA-OAEP(SHA-256). */
async function importRSAPublicKey(pem: string): Promise<CryptoKey> {
  const pemBody = pem
    .replace(/-----BEGIN PUBLIC KEY-----/, "")
    .replace(/-----END PUBLIC KEY-----/, "")
    .replace(/\s/g, "");
  const binaryDer = Uint8Array.from(atob(pemBody), (c) => c.charCodeAt(0));

  return crypto.subtle.importKey(
    "spki",
    binaryDer.buffer,
    { name: "RSA-OAEP", hash: "SHA-256" },
    false,
    ["encrypt"]
  );
}

/** Convert an ArrayBuffer to base64 string. */
function toBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}
