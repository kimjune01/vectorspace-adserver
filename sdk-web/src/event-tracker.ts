/**
 * Event tracking for impression, click, and viewable events.
 */
export class EventTracker {
  constructor(
    private endpoint: string,
    private publisherId?: string,
  ) {}

  /**
   * Report an impression event. Returns false if frequency-capped (429).
   */
  async reportImpression(
    auctionId: number,
    advertiserId: string,
    userId?: string,
  ): Promise<boolean> {
    const body: Record<string, unknown> = {
      auction_id: auctionId,
      advertiser_id: advertiserId,
    };
    if (userId) body.user_id = userId;
    if (this.publisherId) body.publisher_id = this.publisherId;

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

  /** Report a click event. */
  async reportClick(
    auctionId: number,
    advertiserId: string,
    userId?: string,
  ): Promise<void> {
    const body: Record<string, unknown> = {
      auction_id: auctionId,
      advertiser_id: advertiserId,
    };
    if (userId) body.user_id = userId;
    if (this.publisherId) body.publisher_id = this.publisherId;

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

  /** Report a viewable event. */
  async reportViewable(
    auctionId: number,
    advertiserId: string,
    userId?: string,
  ): Promise<void> {
    const body: Record<string, unknown> = {
      auction_id: auctionId,
      advertiser_id: advertiserId,
    };
    if (userId) body.user_id = userId;
    if (this.publisherId) body.publisher_id = this.publisherId;

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
}
