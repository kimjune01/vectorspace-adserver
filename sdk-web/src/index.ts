/**
 * @vectorspace/sdk — Reference Publisher SDK for the Vector Space Exchange.
 *
 * This is the canonical SDK implementation. iOS, Android, and Python SDKs
 * follow the API surface and conventions defined here.
 */

export { VectorSpace } from "./client.js";
export { EmbeddingCache, squaredEuclidean } from "./embedding-cache.js";
export { EventTracker } from "./event-tracker.js";
export { TEEClient, importRSAPublicKey, toBase64 } from "./tee.js";
export { extractIntent } from "./intent.js";
export type {
  VectorSpaceConfig,
  ChatMessage,
  AdRequestOptions,
  AdBidder,
  AdResponse,
  TEEAdResponse,
  CachedEmbedding,
  ProximityResult,
  EncryptedEmbedding,
} from "./types.js";
