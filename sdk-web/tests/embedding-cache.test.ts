import { describe, it, expect, vi, beforeEach } from "vitest";
import { EmbeddingCache, squaredEuclidean } from "../src/embedding-cache.js";

describe("squaredEuclidean", () => {
  it("returns 0 for identical vectors", () => {
    expect(squaredEuclidean([1, 2, 3], [1, 2, 3])).toBe(0);
  });

  it("computes distance correctly", () => {
    // (1-4)^2 + (2-6)^2 = 9 + 16 = 25
    expect(squaredEuclidean([1, 2], [4, 6])).toBe(25);
  });

  it("is symmetric", () => {
    const a = [1, 2, 3];
    const b = [4, 5, 6];
    expect(squaredEuclidean(a, b)).toBe(squaredEuclidean(b, a));
  });
});

describe("EmbeddingCache", () => {
  let cache: EmbeddingCache;

  beforeEach(() => {
    cache = new EmbeddingCache("http://localhost:8080");
    vi.restoreAllMocks();
  });

  it("starts with empty embeddings", () => {
    expect(cache.getEmbeddings()).toEqual([]);
  });

  it("syncs embeddings from server", async () => {
    const embeddings = [
      { id: "adv-1", name: "Foo", embedding: [1, 0], bid_price: 5, sigma: 1, currency: "USD" },
      { id: "adv-2", name: "Bar", embedding: [0, 1], bid_price: 3, sigma: 1, currency: "USD" },
    ];

    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ embeddings }), {
        status: 200,
        headers: { ETag: '"v1"' },
      })
    );

    await cache.sync();
    expect(cache.getEmbeddings()).toEqual(embeddings);
  });

  it("sends If-None-Match on second sync", async () => {
    const embeddings = [
      { id: "adv-1", name: "Foo", embedding: [1, 0], bid_price: 5, sigma: 1, currency: "USD" },
    ];

    const fetchSpy = vi.spyOn(globalThis, "fetch");

    // First sync — returns data
    fetchSpy.mockResolvedValueOnce(
      new Response(JSON.stringify({ embeddings }), {
        status: 200,
        headers: { ETag: '"v1"' },
      })
    );
    await cache.sync();

    // Second sync — returns 304
    fetchSpy.mockResolvedValueOnce(new Response(null, { status: 304 }));
    await cache.sync();

    const secondCall = fetchSpy.mock.calls[1];
    const headers = secondCall[1]?.headers as Record<string, string>;
    expect(headers["If-None-Match"]).toBe('"v1"');
    expect(cache.getEmbeddings()).toEqual(embeddings); // unchanged
  });

  it("throws on non-ok, non-304 response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response("server error", { status: 500 })
    );
    await expect(cache.sync()).rejects.toThrow("Embeddings API error: 500");
  });

  it("returns proximity results sorted by distance", async () => {
    const embeddings = [
      { id: "far", name: "Far", embedding: [10, 0], bid_price: 5, sigma: 1, currency: "USD" },
      { id: "close", name: "Close", embedding: [1, 0], bid_price: 3, sigma: 1, currency: "USD" },
      { id: "mid", name: "Mid", embedding: [5, 0], bid_price: 4, sigma: 1, currency: "USD" },
    ];

    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ embeddings }), {
        status: 200,
        headers: { ETag: '"v1"' },
      })
    );
    await cache.sync();

    const results = cache.proximity([0, 0]);
    expect(results.map((r) => r.id)).toEqual(["close", "mid", "far"]);
    expect(results[0].distance).toBe(1);
    expect(results[1].distance).toBe(25);
    expect(results[2].distance).toBe(100);
  });

  it("returns empty proximity for empty cache", () => {
    expect(cache.proximity([1, 2, 3])).toEqual([]);
  });
});
