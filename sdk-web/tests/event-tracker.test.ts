import { describe, it, expect, vi, beforeEach } from "vitest";
import { EventTracker } from "../src/event-tracker.js";

describe("EventTracker", () => {
  let tracker: EventTracker;
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    tracker = new EventTracker("http://localhost:8080", "pub-1");
    fetchSpy = vi.spyOn(globalThis, "fetch");
  });

  describe("reportImpression", () => {
    it("sends impression event and returns true", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      const result = await tracker.reportImpression(42, "adv-1", "user-1");

      expect(result).toBe(true);
      const [url, opts] = fetchSpy.mock.calls[0];
      expect(url).toBe("http://localhost:8080/event/impression");
      expect(opts?.method).toBe("POST");
      const body = JSON.parse(opts?.body as string);
      expect(body).toEqual({
        auction_id: 42,
        advertiser_id: "adv-1",
        user_id: "user-1",
        publisher_id: "pub-1",
      });
    });

    it("returns false on 429 (frequency capped)", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("too many", { status: 429 }));

      const result = await tracker.reportImpression(42, "adv-1");
      expect(result).toBe(false);
    });

    it("throws on other errors", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("server error", { status: 500 })
      );

      await expect(tracker.reportImpression(42, "adv-1")).rejects.toThrow(
        "server error"
      );
    });

    it("omits user_id when not provided", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await tracker.reportImpression(42, "adv-1");
      const body = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(body).not.toHaveProperty("user_id");
    });
  });

  describe("reportClick", () => {
    it("sends click event", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await tracker.reportClick(42, "adv-1", "user-1");

      const [url, opts] = fetchSpy.mock.calls[0];
      expect(url).toBe("http://localhost:8080/event/click");
      const body = JSON.parse(opts?.body as string);
      expect(body.auction_id).toBe(42);
      expect(body.advertiser_id).toBe("adv-1");
    });

    it("throws on error", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("not found", { status: 404 })
      );

      await expect(tracker.reportClick(42, "adv-1")).rejects.toThrow(
        "not found"
      );
    });
  });

  describe("reportViewable", () => {
    it("sends viewable event", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await tracker.reportViewable(42, "adv-1");

      const [url] = fetchSpy.mock.calls[0];
      expect(url).toBe("http://localhost:8080/event/viewable");
    });

    it("throws on error", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("bad request", { status: 400 })
      );

      await expect(tracker.reportViewable(42, "adv-1")).rejects.toThrow(
        "bad request"
      );
    });
  });

  describe("without publisherId", () => {
    it("omits publisher_id from body", async () => {
      const trackerNoPub = new EventTracker("http://localhost:8080");
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await trackerNoPub.reportImpression(42, "adv-1");
      const body = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(body).not.toHaveProperty("publisher_id");
    });
  });
});
