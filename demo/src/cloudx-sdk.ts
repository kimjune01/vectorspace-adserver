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

const INTENT_PROMPT = `Given a conversation, describe what kind of professional service would help this person — as a single natural sentence in the style an advertiser would use to describe their own practice.

Focus on the service, its value proposition, and who it serves. Do NOT extract demographics or personal data about the user.

Examples:
- "Licensed physical therapist specializing in sports injury rehabilitation for climbers with finger pulley strains"
- "Family law attorney helping parents navigate custody arrangements and mediation during divorce proceedings"
- "Certified financial planner guiding first-time investors through retirement planning and portfolio diversification"
- "Reading specialist providing structured literacy tutoring for elementary students with learning differences"
- "Full-stack development consultancy helping early-stage startups build and ship their MVP"
- "Certified dog behaviorist working with reactive and fearful rescue dogs through desensitization programs"

Respond with ONLY the service description sentence, nothing else.`;

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
