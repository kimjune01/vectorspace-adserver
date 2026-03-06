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

export interface ProximityResult {
  id: string;
  distance: number;
}

export class CloudX {
  private endpoint: string;
  private embeddingCache: { id: string; embedding: number[] }[] = [];
  private embeddingEtag: string | null = null;

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
