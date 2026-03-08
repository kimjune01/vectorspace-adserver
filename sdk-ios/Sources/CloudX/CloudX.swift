// Reference SDK: sdk-web/ (TypeScript). This SDK follows the API surface defined there.
import Foundation
#if canImport(UIKit)
import UIKit
#endif

/// CloudX Publisher SDK for iOS.
///
/// Provides ad requesting, embedding-based proximity search, event tracking,
/// and UIKit viewability observation.
///
/// Usage:
/// ```swift
/// let client = CloudX(endpoint: "http://localhost:8080")
/// try await client.syncEmbeddings()
/// let ad = try await client.requestAd(intent: "back pain relief")
/// ```
public final class CloudX: @unchecked Sendable {
    private let endpoint: String
    private let session: URLSession
    private let embeddingCache: EmbeddingCache
    let eventTracker: EventTracker

    #if canImport(UIKit)
    /// Retains active viewability observers so they aren't deallocated.
    private var viewabilityObservers: [ObjectIdentifier: ViewabilityObserver] = [:]
    private let observerLock = NSLock()
    #endif

    /// Creates a new CloudX client.
    /// - Parameter endpoint: Base URL of the CloudX ad server (e.g. "http://localhost:8080").
    public init(endpoint: String) {
        let trimmed = endpoint.hasSuffix("/")
            ? String(endpoint.dropLast())
            : endpoint
        self.endpoint = trimmed
        self.session = URLSession.shared
        self.embeddingCache = EmbeddingCache(endpoint: trimmed, session: URLSession.shared)
        self.eventTracker = EventTracker(endpoint: trimmed, session: URLSession.shared)
    }

    /// Creates a new CloudX client with a custom URLSession (useful for testing).
    init(endpoint: String, session: URLSession) {
        let trimmed = endpoint.hasSuffix("/")
            ? String(endpoint.dropLast())
            : endpoint
        self.endpoint = trimmed
        self.session = session
        self.embeddingCache = EmbeddingCache(endpoint: trimmed, session: session)
        self.eventTracker = EventTracker(endpoint: trimmed, session: session)
    }

    // MARK: - Embeddings

    /// Syncs the local embedding cache from the server.
    /// Uses ETag/If-None-Match so repeated calls only transfer data when it changes.
    public func syncEmbeddings() async throws {
        try await embeddingCache.sync()
    }

    /// Computes squared Euclidean distance from `queryEmbedding` to each cached
    /// advertiser embedding. Returns results sorted by distance ascending (closest first).
    ///
    /// Call `syncEmbeddings()` first to populate the cache.
    public func proximity(queryEmbedding: [Float]) -> [ProximityResult] {
        return embeddingCache.proximity(queryEmbedding: queryEmbedding)
    }

    /// Embeds arbitrary text via the server's embedding sidecar.
    /// - Parameter text: The text to embed.
    /// - Returns: The embedding vector.
    public func embed(text: String) async throws -> [Float] {
        guard let url = URL(string: "\(endpoint)/embed") else {
            throw CloudXError.invalidURL("\(endpoint)/embed")
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(["text": text])

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw CloudXError.httpError(statusCode: 0, body: "No HTTP response")
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw CloudXError.httpError(statusCode: httpResponse.statusCode, body: body)
        }

        let decoded = try JSONDecoder().decode(EmbedResponse.self, from: data)
        return decoded.embedding
    }

    // MARK: - Ad Requests

    /// Requests an ad for the given intent.
    /// - Parameters:
    ///   - intent: The user's search intent / context string.
    ///   - tau: Optional relevance threshold. Only ads with squared distance below tau are eligible.
    /// - Returns: The ad response, or `nil` if no bidders passed the relevance threshold.
    public func requestAd(intent: String, tau: Double? = nil) async throws -> AdResponse? {
        guard let url = URL(string: "\(endpoint)/ad-request") else {
            throw CloudXError.invalidURL("\(endpoint)/ad-request")
        }

        var payload: [String: Any] = ["intent": intent]
        if let tau, tau > 0 {
            payload["tau"] = tau
        }

        let jsonData = try JSONSerialization.data(withJSONObject: payload)

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = jsonData

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw CloudXError.httpError(statusCode: 0, body: "No HTTP response")
        }

        if httpResponse.statusCode == 500 {
            let body = String(data: data, encoding: .utf8) ?? ""
            if body.contains("no bidders passed") {
                return nil
            }
            throw CloudXError.httpError(statusCode: 500, body: body)
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw CloudXError.httpError(statusCode: httpResponse.statusCode, body: body)
        }

        do {
            let decoded = try JSONDecoder().decode(AdResponse.self, from: data)
            return decoded
        } catch {
            throw CloudXError.decodingError(error)
        }
    }

    // MARK: - Event Tracking

    /// Reports an impression event.
    /// - Returns: `true` on success, `false` if frequency-capped (429).
    @discardableResult
    public func reportImpression(auctionId: Int, advertiserId: String, userId: String? = nil) async throws -> Bool {
        return try await eventTracker.reportImpression(
            auctionId: auctionId,
            advertiserId: advertiserId,
            userId: userId
        )
    }

    /// Reports a click event.
    public func reportClick(auctionId: Int, advertiserId: String, userId: String? = nil) async throws {
        try await eventTracker.reportClick(
            auctionId: auctionId,
            advertiserId: advertiserId,
            userId: userId
        )
    }

    /// Reports a viewable event.
    public func reportViewable(auctionId: Int, advertiserId: String, userId: String? = nil) async throws {
        try await eventTracker.reportViewable(
            auctionId: auctionId,
            advertiserId: advertiserId,
            userId: userId
        )
    }

    // MARK: - Viewability

    #if canImport(UIKit)
    /// Starts observing a UIView for IAB viewability (50% visible for 1+ second).
    /// When the condition is met, automatically fires `reportViewable`.
    ///
    /// The observer is retained internally and cleaned up after firing or when
    /// `stopObservingViewability(for:)` is called.
    ///
    /// - Parameters:
    ///   - view: The UIView to observe.
    ///   - auctionId: The auction ID from the ad response.
    ///   - advertiserId: The winning advertiser's ID.
    ///   - userId: Optional user identifier.
    public func observeViewability(view: UIView, auctionId: Int, advertiserId: String, userId: String? = nil) {
        let observer = ViewabilityObserver(
            view: view,
            auctionId: auctionId,
            advertiserId: advertiserId,
            userId: userId,
            tracker: eventTracker
        )
        let key = ObjectIdentifier(view)
        observerLock.lock()
        viewabilityObservers[key] = observer
        observerLock.unlock()
        observer.start()
    }

    /// Stops viewability observation for the given view.
    public func stopObservingViewability(for view: UIView) {
        let key = ObjectIdentifier(view)
        observerLock.lock()
        let observer = viewabilityObservers.removeValue(forKey: key)
        observerLock.unlock()
        observer?.stop()
    }
    #endif
}
