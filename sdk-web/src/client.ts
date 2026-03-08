/**
 * Vector Space Exchange — Reference Publisher SDK
 *
 * Usage:
 *   const vs = new VectorSpace({ endpoint: "https://api.vectorspace.exchange" });
 *   const ad = await vs.requestAd({ intent: "dog training near me", tau: 0.6 });
 *   if (ad) renderAd(ad.winner);
 *
 *   // Or from a chat conversation:
 *   const ad = await vs.requestAdFromChat(messages, 0.6);
 */

import { EmbeddingCache } from "./embedding-cache.js";
import { EventTracker } from "./event-tracker.js";
import { TEEClient } from "./tee.js";
import { extractIntent } from "./intent.js";
import type {
  VectorSpaceConfig,
  ChatMessage,
  AdRequestOptions,
  AdResponse,
  TEEAdResponse,
  ProximityResult,
} from "./types.js";

export class VectorSpace {
  private endpoint: string;
  private publisherId?: string;
  private embeddingCache: EmbeddingCache;
  private eventTracker: EventTracker;
  private teeClient: TEEClient;

  constructor(config: VectorSpaceConfig) {
    this.endpoint = config.endpoint.replace(/\/+$/, "");
    this.publisherId = config.publisherId;
    this.embeddingCache = new EmbeddingCache(this.endpoint);
    this.eventTracker = new EventTracker(this.endpoint, this.publisherId);
    this.teeClient = new TEEClient(this.endpoint);
  }

  // ── Embeddings ──────────────────────────────────────────────────

  /** Sync advertiser embeddings from the server (ETag caching). */
  async syncEmbeddings(): Promise<void> {
    return this.embeddingCache.sync();
  }

  /** Embed arbitrary text via the server's embedding sidecar. */
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

  /** Compute squared Euclidean distance to all cached advertisers. Sorted ascending (closest first). */
  proximity(query: number[]): ProximityResult[] {
    return this.embeddingCache.proximity(query);
  }

  // ── Ad Requests ─────────────────────────────────────────────────

  /** Request an ad for the given intent. Returns null if no ad passed the relevance gate. */
  async requestAd(options: AdRequestOptions): Promise<AdResponse | null> {
    const body: Record<string, unknown> = { intent: options.intent };
    if (options.tau != null && options.tau > 0) {
      body.tau = options.tau;
    }
    if (this.publisherId) {
      body.publisher_id = this.publisherId;
    }

    const resp = await fetch(`${this.endpoint}/ad-request`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      if (resp.status === 500 && text.includes("no bidders passed")) {
        return null;
      }
      throw new Error(text || `Ad request error ${resp.status}`);
    }

    return resp.json();
  }

  /** Extract intent from a chat conversation via the server's /chat endpoint. */
  async extractIntent(messages: ChatMessage[]): Promise<string> {
    return extractIntent(this.endpoint, messages);
  }

  /** Extract intent + request ad in one call. Returns null if intent is "NONE" or no ad matches. */
  async requestAdFromChat(
    messages: ChatMessage[],
    tau?: number,
  ): Promise<AdResponse | null> {
    const intent = await this.extractIntent(messages);
    if (intent === "NONE") return null;
    return this.requestAd({ intent, tau });
  }

  // ── TEE-Encrypted Auction ──────────────────────────────────────

  /** Fetch the enclave's attested public key. */
  async fetchAttestation(): Promise<void> {
    return this.teeClient.fetchAttestation();
  }

  /** TEE-private ad request: embed intent locally, encrypt with enclave key, send ciphertext. */
  async requestAdTEE(
    options: AdRequestOptions,
  ): Promise<TEEAdResponse | null> {
    if (!this.teeClient.hasAttestation) {
      await this.teeClient.fetchAttestation();
    }

    const embedding = await this.embed(options.intent);
    const encrypted = await this.teeClient.encryptEmbedding(embedding);

    const body: Record<string, unknown> = {
      encrypted_embedding: encrypted,
    };
    if (options.tau != null && options.tau > 0) {
      body.tau = options.tau;
    }
    if (this.publisherId) {
      body.publisher_id = this.publisherId;
    }

    const resp = await fetch(`${this.endpoint}/ad-request`, {
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

  // ── Event Tracking ─────────────────────────────────────────────

  /** Report impression (returns false if frequency-capped 429). */
  async reportImpression(
    auctionId: number,
    advertiserId: string,
    userId?: string,
  ): Promise<boolean> {
    return this.eventTracker.reportImpression(auctionId, advertiserId, userId);
  }

  /** Report click. */
  async reportClick(
    auctionId: number,
    advertiserId: string,
    userId?: string,
  ): Promise<void> {
    return this.eventTracker.reportClick(auctionId, advertiserId, userId);
  }

  /** Report viewable. */
  async reportViewable(
    auctionId: number,
    advertiserId: string,
    userId?: string,
  ): Promise<void> {
    return this.eventTracker.reportViewable(auctionId, advertiserId, userId);
  }
}
