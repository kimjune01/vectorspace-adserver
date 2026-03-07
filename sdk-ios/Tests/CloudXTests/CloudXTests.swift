import XCTest
@testable import CloudX

// MARK: - Model decoding tests

final class ModelDecodingTests: XCTestCase {

    func testAdBidderDecoding() throws {
        let json = """
        {
            "id": "adv-1",
            "rank": 1,
            "name": "Acme Health",
            "intent": "back pain",
            "bid_price": 2.50,
            "sigma": 0.30,
            "score": 1.85,
            "distance_sq": 0.0412,
            "log_bid": 0.916
        }
        """.data(using: .utf8)!

        let bidder = try JSONDecoder().decode(AdBidder.self, from: json)
        XCTAssertEqual(bidder.id, "adv-1")
        XCTAssertEqual(bidder.rank, 1)
        XCTAssertEqual(bidder.name, "Acme Health")
        XCTAssertEqual(bidder.intent, "back pain")
        XCTAssertEqual(bidder.bidPrice, 2.50, accuracy: 0.001)
        XCTAssertEqual(bidder.sigma, 0.30, accuracy: 0.001)
        XCTAssertEqual(bidder.score, 1.85, accuracy: 0.001)
        XCTAssertEqual(bidder.distanceSq, 0.0412, accuracy: 0.0001)
        XCTAssertEqual(bidder.logBid, 0.916, accuracy: 0.001)
    }

    func testAdResponseDecoding() throws {
        let json = """
        {
            "auction_id": 42,
            "intent": "back pain relief",
            "winner": {
                "id": "adv-1",
                "rank": 1,
                "name": "Acme Health",
                "intent": "back pain",
                "bid_price": 2.50,
                "sigma": 0.30,
                "score": 1.85,
                "distance_sq": 0.04,
                "log_bid": 0.92
            },
            "runner_up": {
                "id": "adv-2",
                "rank": 2,
                "name": "Beta Clinic",
                "intent": "spine care",
                "bid_price": 1.80,
                "sigma": 0.55,
                "score": 1.20,
                "distance_sq": 0.15,
                "log_bid": 0.59
            },
            "all_bidders": [
                {
                    "id": "adv-1",
                    "rank": 1,
                    "name": "Acme Health",
                    "intent": "back pain",
                    "bid_price": 2.50,
                    "sigma": 0.30,
                    "score": 1.85,
                    "distance_sq": 0.04,
                    "log_bid": 0.92
                },
                {
                    "id": "adv-2",
                    "rank": 2,
                    "name": "Beta Clinic",
                    "intent": "spine care",
                    "bid_price": 1.80,
                    "sigma": 0.55,
                    "score": 1.20,
                    "distance_sq": 0.15,
                    "log_bid": 0.59
                }
            ],
            "payment": 1.80,
            "currency": "USD",
            "bid_count": 2,
            "eligible_count": 2
        }
        """.data(using: .utf8)!

        let response = try JSONDecoder().decode(AdResponse.self, from: json)
        XCTAssertEqual(response.auctionId, 42)
        XCTAssertEqual(response.intent, "back pain relief")
        XCTAssertEqual(response.winner?.id, "adv-1")
        XCTAssertEqual(response.runnerUp?.id, "adv-2")
        XCTAssertEqual(response.allBidders.count, 2)
        XCTAssertEqual(response.payment, 1.80, accuracy: 0.001)
        XCTAssertEqual(response.currency, "USD")
        XCTAssertEqual(response.bidCount, 2)
        XCTAssertEqual(response.eligibleCount, 2)
    }

    func testAdResponseDecodingNoRunnerUp() throws {
        let json = """
        {
            "auction_id": 1,
            "intent": "test",
            "winner": {
                "id": "adv-1",
                "rank": 1,
                "name": "Solo Ad",
                "intent": "test",
                "bid_price": 1.00,
                "sigma": 0.30,
                "score": 1.00,
                "distance_sq": 0.01,
                "log_bid": 0.00
            },
            "all_bidders": [
                {
                    "id": "adv-1",
                    "rank": 1,
                    "name": "Solo Ad",
                    "intent": "test",
                    "bid_price": 1.00,
                    "sigma": 0.30,
                    "score": 1.00,
                    "distance_sq": 0.01,
                    "log_bid": 0.00
                }
            ],
            "payment": 0.00,
            "currency": "USD",
            "bid_count": 1,
            "eligible_count": 1
        }
        """.data(using: .utf8)!

        let response = try JSONDecoder().decode(AdResponse.self, from: json)
        XCTAssertEqual(response.auctionId, 1)
        XCTAssertNotNil(response.winner)
        XCTAssertNil(response.runnerUp)
        XCTAssertEqual(response.allBidders.count, 1)
    }

    func testEmbeddingsResponseDecoding() throws {
        let json = """
        {
            "version": "abc123",
            "embeddings": [
                {"id": "adv-1", "embedding": [0.1, 0.2, 0.3]},
                {"id": "adv-2", "embedding": [0.9, 0.8, 0.7]}
            ]
        }
        """.data(using: .utf8)!

        let response = try JSONDecoder().decode(EmbeddingsResponse.self, from: json)
        XCTAssertEqual(response.version, "abc123")
        XCTAssertEqual(response.embeddings.count, 2)
        XCTAssertEqual(response.embeddings[0].id, "adv-1")
        XCTAssertEqual(response.embeddings[0].embedding, [0.1, 0.2, 0.3])
        XCTAssertEqual(response.embeddings[1].id, "adv-2")
        XCTAssertEqual(response.embeddings[1].embedding, [0.9, 0.8, 0.7])
    }

    func testEventBodyEncoding() throws {
        let body = EventBody(auctionId: 42, advertiserId: "adv-1", userId: "user-99")
        let data = try JSONEncoder().encode(body)
        let dict = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        XCTAssertEqual(dict["auction_id"] as? Int, 42)
        XCTAssertEqual(dict["advertiser_id"] as? String, "adv-1")
        XCTAssertEqual(dict["user_id"] as? String, "user-99")
    }

    func testEventBodyEncodingNilUserId() throws {
        let body = EventBody(auctionId: 1, advertiserId: "adv-1", userId: nil)
        let data = try JSONEncoder().encode(body)
        let dict = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        XCTAssertEqual(dict["auction_id"] as? Int, 1)
        XCTAssertEqual(dict["advertiser_id"] as? String, "adv-1")
        XCTAssertNil(dict["user_id"])
    }
}

// MARK: - Proximity / distance tests

final class ProximityTests: XCTestCase {

    func testSquaredEuclideanDistanceIdentical() {
        let a: [Float] = [1.0, 2.0, 3.0]
        let dist = squaredEuclideanDistance(a, a)
        XCTAssertEqual(dist, 0.0, accuracy: 1e-6)
    }

    func testSquaredEuclideanDistanceKnownValue() {
        // ||[0.1, 0.2, 0.3] - [0.9, 0.8, 0.7]||^2 = 0.64 + 0.36 + 0.16 = 1.16
        let a: [Float] = [0.1, 0.2, 0.3]
        let b: [Float] = [0.9, 0.8, 0.7]
        let dist = squaredEuclideanDistance(a, b)
        XCTAssertEqual(dist, 1.16, accuracy: 1e-4)
    }

    func testSquaredEuclideanDistanceSymmetric() {
        let a: [Float] = [0.5, 0.1, 0.9]
        let b: [Float] = [0.2, 0.4, 0.6]
        let distAB = squaredEuclideanDistance(a, b)
        let distBA = squaredEuclideanDistance(b, a)
        XCTAssertEqual(distAB, distBA, accuracy: 1e-6)
    }

    func testSquaredEuclideanDistanceEmpty() {
        let dist = squaredEuclideanDistance([], [])
        XCTAssertEqual(dist, 0.0)
    }

    func testProximityEmptyCache() {
        let cache = EmbeddingCache(endpoint: "http://unused", session: .shared)
        let results = cache.proximity(queryEmbedding: [0.5, 0.5, 0.5])
        XCTAssertTrue(results.isEmpty)
    }
}

// MARK: - CloudX client init tests

final class CloudXInitTests: XCTestCase {

    func testEndpointTrailingSlashTrimmed() {
        let client = CloudX(endpoint: "http://localhost:8080/")
        // Verify it doesn't crash and produces a usable client.
        // The trailing slash is trimmed internally.
        let results = client.proximity(queryEmbedding: [0.1, 0.2])
        XCTAssertTrue(results.isEmpty) // no cache synced yet
    }

    func testEndpointNoTrailingSlash() {
        let client = CloudX(endpoint: "http://localhost:8080")
        let results = client.proximity(queryEmbedding: [0.1, 0.2])
        XCTAssertTrue(results.isEmpty)
    }
}

// MARK: - CloudXError tests

final class CloudXErrorTests: XCTestCase {

    func testErrorDescriptions() {
        let urlError = CloudXError.invalidURL("bad://url")
        XCTAssertTrue(urlError.localizedDescription.contains("Invalid URL"))

        let httpError = CloudXError.httpError(statusCode: 500, body: "internal error")
        XCTAssertTrue(httpError.localizedDescription.contains("500"))

        let noBidders = CloudXError.noBidders
        XCTAssertTrue(noBidders.localizedDescription.contains("No bidders"))
    }
}
