import type { CachedEmbedding, ProximityResult } from "./types.js";

/** Squared Euclidean distance: ||a - b||² */
export function squaredEuclidean(a: number[], b: number[]): number {
  let sum = 0;
  for (let i = 0; i < a.length; i++) {
    const d = a[i] - b[i];
    sum += d * d;
  }
  return sum;
}

/**
 * ETag-based embedding cache.
 * Syncs advertiser embeddings from the server and provides local proximity search.
 */
export class EmbeddingCache {
  private embeddings: CachedEmbedding[] = [];
  private etag: string | null = null;

  constructor(private endpoint: string) {}

  /** Current cached embeddings. */
  getEmbeddings(): CachedEmbedding[] {
    return this.embeddings;
  }

  /**
   * Sync embeddings from the server.
   * Uses If-None-Match / ETag for efficient caching — 304 means no update needed.
   */
  async sync(): Promise<void> {
    const headers: Record<string, string> = {};
    if (this.etag) {
      headers["If-None-Match"] = this.etag;
    }

    const resp = await fetch(`${this.endpoint}/embeddings`, { headers });
    if (resp.status === 304) return;

    if (!resp.ok) {
      throw new Error(`Embeddings API error: ${resp.status}`);
    }

    const data = await resp.json();
    this.embeddings = data.embeddings;
    this.etag = resp.headers.get("ETag");
  }

  /**
   * Compute squared Euclidean distance from a query embedding to all cached
   * advertiser embeddings. Returns results sorted ascending (closest first).
   */
  proximity(queryEmbedding: number[]): ProximityResult[] {
    return this.embeddings
      .map((entry) => ({
        id: entry.id,
        distance: squaredEuclidean(queryEmbedding, entry.embedding),
      }))
      .sort((a, b) => a.distance - b.distance);
  }
}
