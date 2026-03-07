import Foundation

// MARK: - Embedding types

/// A single advertiser embedding entry returned by GET /embeddings.
public struct EmbeddingEntry: Codable, Sendable {
    public let id: String
    public let embedding: [Float]
}

/// Response from GET /embeddings.
public struct EmbeddingsResponse: Codable, Sendable {
    public let version: String
    public let embeddings: [EmbeddingEntry]
}

/// Response from POST /embed.
struct EmbedResponse: Codable {
    let embedding: [Float]
}

// MARK: - Ad request / response

/// A single bidder in the auction response.
public struct AdBidder: Codable, Sendable {
    public let id: String
    public let rank: Int
    public let name: String
    public let intent: String
    public let bidPrice: Double
    public let sigma: Double
    public let score: Double
    public let distanceSq: Double
    public let logBid: Double

    enum CodingKeys: String, CodingKey {
        case id, rank, name, intent, sigma, score
        case bidPrice = "bid_price"
        case distanceSq = "distance_sq"
        case logBid = "log_bid"
    }
}

/// Response from POST /ad-request.
public struct AdResponse: Codable, Sendable {
    public let auctionId: Int
    public let intent: String
    public let winner: AdBidder?
    public let runnerUp: AdBidder?
    public let allBidders: [AdBidder]
    public let payment: Double
    public let currency: String
    public let bidCount: Int
    public let eligibleCount: Int

    enum CodingKeys: String, CodingKey {
        case intent, winner, payment, currency
        case auctionId = "auction_id"
        case runnerUp = "runner_up"
        case allBidders = "all_bidders"
        case bidCount = "bid_count"
        case eligibleCount = "eligible_count"
    }
}

// MARK: - Proximity

/// Result of a local proximity search against cached embeddings.
public struct ProximityResult: Sendable {
    public let id: String
    public let distance: Float
}

// MARK: - Event tracking

/// Body sent to POST /event/*.
struct EventBody: Encodable {
    let auctionId: Int
    let advertiserId: String
    let userId: String?

    enum CodingKeys: String, CodingKey {
        case auctionId = "auction_id"
        case advertiserId = "advertiser_id"
        case userId = "user_id"
    }
}

// MARK: - Errors

/// Errors thrown by the CloudX SDK.
public enum CloudXError: Error, LocalizedError {
    case invalidURL(String)
    case httpError(statusCode: Int, body: String)
    case decodingError(Error)
    case noBidders

    public var errorDescription: String? {
        switch self {
        case .invalidURL(let url):
            return "Invalid URL: \(url)"
        case .httpError(let code, let body):
            return "HTTP \(code): \(body)"
        case .decodingError(let error):
            return "Decoding error: \(error.localizedDescription)"
        case .noBidders:
            return "No bidders passed the relevance threshold"
        }
    }
}
