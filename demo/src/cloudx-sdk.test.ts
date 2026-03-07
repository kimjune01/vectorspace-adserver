import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { CloudX } from "./cloudx-sdk";

// Mock server
let embeddingsCallCount = 0;
const handlers = [
  http.get("http://test-endpoint/embeddings", ({ request }) => {
    embeddingsCallCount++;
    const etag = '"abc123"';
    const ifNoneMatch = request.headers.get("If-None-Match");
    if (ifNoneMatch === etag) {
      return new HttpResponse(null, { status: 304 });
    }
    return HttpResponse.json(
      {
        version: "abc123",
        embeddings: [
          { id: "adv-1", embedding: [0.1, 0.2, 0.3] },
          { id: "adv-2", embedding: [0.9, 0.8, 0.7] },
        ],
      },
      { headers: { ETag: etag } }
    );
  }),

  http.post("http://test-endpoint/embed", async ({ request }) => {
    const body = (await request.json()) as { text: string };
    if (!body.text) {
      return new HttpResponse("text is required", { status: 400 });
    }
    return HttpResponse.json({ embedding: [0.5, 0.5, 0.5] });
  }),

  http.post("http://test-endpoint/chat", async ({ request }) => {
    const body = (await request.json()) as {
      messages: { role: string; content: string }[];
      system?: string;
    };
    const userMsg = body.messages.find((m) => m.role === "user");
    const content = userMsg?.content ?? "";

    // Simulate NONE for casual queries
    if (
      content.includes("movie") ||
      content.includes("joke") ||
      content.includes("how's it going")
    ) {
      return HttpResponse.json({ content: "NONE" });
    }

    // Simulate a service description for real queries
    return HttpResponse.json({
      content:
        "Physical therapist providing treatment for back pain and posture issues.",
    });
  }),

  http.post("http://test-endpoint/ad-request", async ({ request }) => {
    const body = (await request.json()) as {
      intent: string;
      tau?: number;
    };

    if (!body.intent) {
      return new HttpResponse("missing intent", { status: 400 });
    }

    return HttpResponse.json({
      intent: body.intent,
      winner: {
        id: "adv-1",
        rank: 1,
        name: "Test Advertiser",
        intent: "test advertiser intent",
        bid_price: 2.0,
        sigma: 0.5,
        score: -0.5,
        distance_sq: 0.3,
        log_bid: 0.69,
      },
      runner_up: null,
      all_bidders: [
        {
          id: "adv-1",
          rank: 1,
          name: "Test Advertiser",
          intent: "test advertiser intent",
          bid_price: 2.0,
          sigma: 0.5,
          score: -0.5,
          distance_sq: 0.3,
          log_bid: 0.69,
        },
      ],
      payment: 1.5,
      currency: "USD",
      bid_count: 1,
      eligible_count: 1,
    });
  }),
];

const server = setupServer(...handlers);

beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  embeddingsCallCount = 0;
});
afterAll(() => server.close());

const cloudx = new CloudX({ endpoint: "http://test-endpoint" });

describe("CloudX SDK", () => {
  describe("extractIntent", () => {
    it("returns a service description for relevant queries", async () => {
      const intent = await cloudx.extractIntent([
        { role: "user", content: "my back hurts from sitting all day" },
      ]);
      expect(intent).toContain("Physical therapist");
    });

    it("returns NONE for casual queries", async () => {
      const intent = await cloudx.extractIntent([
        { role: "user", content: "what's the best movie lately" },
      ]);
      expect(intent).toBe("NONE");
    });
  });

  describe("requestAd", () => {
    it("returns an ad response with winner", async () => {
      const result = await cloudx.requestAd({ intent: "back pain treatment" });
      expect(result).not.toBeNull();
      expect(result!.winner).not.toBeNull();
      expect(result!.winner!.name).toBe("Test Advertiser");
      expect(result!.payment).toBe(1.5);
      expect(result!.intent).toBe("back pain treatment");
    });

    it("passes tau to the request", async () => {
      const result = await cloudx.requestAd({
        intent: "back pain",
        tau: 0.8,
      });
      expect(result).not.toBeNull();
    });
  });

  describe("requestAdFromChat", () => {
    it("returns an ad for relevant conversations", async () => {
      const result = await cloudx.requestAdFromChat([
        { role: "user", content: "my back hurts from sitting all day" },
      ]);
      expect(result).not.toBeNull();
      expect(result!.winner!.name).toBe("Test Advertiser");
    });

    it("returns null for casual conversations", async () => {
      const result = await cloudx.requestAdFromChat([
        { role: "user", content: "tell me a joke" },
      ]);
      expect(result).toBeNull();
    });

    it("returns null for off-topic conversations", async () => {
      const result = await cloudx.requestAdFromChat([
        { role: "user", content: "what's the best movie lately" },
      ]);
      expect(result).toBeNull();
    });
  });

  describe("syncEmbeddings", () => {
    it("fetches embeddings on first call", async () => {
      await cloudx.syncEmbeddings();
      expect(embeddingsCallCount).toBe(1);
      // proximity should now work with cached data
      const results = cloudx.proximity([0.1, 0.2, 0.3]);
      expect(results.length).toBe(2);
      expect(results[0].id).toBe("adv-1");
    });

    it("sends If-None-Match on subsequent calls", async () => {
      await cloudx.syncEmbeddings(); // first call — gets ETag
      await cloudx.syncEmbeddings(); // second call — sends If-None-Match → 304
      expect(embeddingsCallCount).toBe(2);
      // Cache should still work
      const results = cloudx.proximity([0.1, 0.2, 0.3]);
      expect(results.length).toBe(2);
    });
  });

  describe("embed", () => {
    it("returns an embedding vector", async () => {
      const vec = await cloudx.embed("back pain from sitting");
      expect(vec).toEqual([0.5, 0.5, 0.5]);
    });
  });

  describe("proximity", () => {
    it("returns empty array when cache is empty", () => {
      const fresh = new CloudX({ endpoint: "http://test-endpoint" });
      expect(fresh.proximity([0.5, 0.5, 0.5])).toEqual([]);
    });

    it("sorts by squared Euclidean distance ascending", async () => {
      await cloudx.syncEmbeddings();
      // adv-1 is at [0.1, 0.2, 0.3], adv-2 is at [0.9, 0.8, 0.7]
      // query at [0.1, 0.2, 0.3] → distance to adv-1 = 0, to adv-2 = 0.64+0.36+0.16 = 1.16
      const results = cloudx.proximity([0.1, 0.2, 0.3]);
      expect(results[0].id).toBe("adv-1");
      expect(results[0].distance).toBeCloseTo(0, 5);
      expect(results[1].id).toBe("adv-2");
      expect(results[1].distance).toBeCloseTo(1.16, 2);
    });

    it("returns closest advertiser first for a different query", async () => {
      await cloudx.syncEmbeddings();
      // query near adv-2 at [0.9, 0.8, 0.7]
      const results = cloudx.proximity([0.9, 0.8, 0.7]);
      expect(results[0].id).toBe("adv-2");
      expect(results[0].distance).toBeCloseTo(0, 5);
    });
  });
});
