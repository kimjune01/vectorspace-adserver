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
}

export interface AdResponse {
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

export class CloudX {
  private endpoint: string;

  constructor(config: CloudXConfig) {
    this.endpoint = config.endpoint.replace(/\/+$/, "");
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
}
