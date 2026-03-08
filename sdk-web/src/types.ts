/**
 * Vector Space Exchange — SDK type definitions.
 * This is the reference SDK. iOS, Android, and Python SDKs follow these types.
 */

/** SDK configuration. */
export interface VectorSpaceConfig {
  /** Ad exchange endpoint (no trailing slash). */
  endpoint: string;
  /** Publisher ID — auto-injected into all requests and events. */
  publisherId?: string;
}

/** A single chat message for intent extraction. */
export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

/** Options for requesting an ad. */
export interface AdRequestOptions {
  /** The user's intent — what they're looking for. */
  intent: string;
  /** Relevance threshold (distance² cutoff). Lower = stricter. Omit to allow all. */
  tau?: number;
}

/** A single bidder in an auction result. */
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
  click_url?: string;
  ad_title?: string;
  ad_subtitle?: string;
}

/** Server auction response. */
export interface AdResponse {
  auction_id: number;
  intent: string;
  winner: AdBidder | null;
  runner_up: AdBidder | null;
  all_bidders: AdBidder[];
  payment: number;
  currency: string;
  bid_count: number;
  eligible_count: number;
}

/** TEE-encrypted auction response (reduced fields — enclave doesn't return full bidder details). */
export interface TEEAdResponse {
  auction_id: number;
  winner_id: string;
  payment: number;
  currency: string;
  bid_count: number;
}

/** A cached advertiser embedding from GET /embeddings. */
export interface CachedEmbedding {
  id: string;
  name: string;
  embedding: number[];
  bid_price: number;
  sigma: number;
  currency: string;
}

/** Proximity result from local distance computation. */
export interface ProximityResult {
  id: string;
  distance: number;
}

/** Encrypted embedding payload for TEE auction. */
export interface EncryptedEmbedding {
  aes_key_encrypted: string;
  encrypted_payload: string;
  nonce: string;
  hash_algorithm: string;
}
