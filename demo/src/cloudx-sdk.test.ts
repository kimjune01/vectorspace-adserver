import { describe, it, expect, beforeAll, afterAll, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { CloudX } from "./cloudx-sdk";

// Mock server
const handlers = [
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
afterEach(() => server.resetHandlers());
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
});
