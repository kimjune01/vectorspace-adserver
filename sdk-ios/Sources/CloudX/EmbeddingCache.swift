import Foundation
import Accelerate

/// Manages a local cache of advertiser embeddings with ETag-based HTTP caching.
/// Provides fast proximity search using the Accelerate framework.
public final class EmbeddingCache: @unchecked Sendable {
    private let endpoint: String
    private let session: URLSession

    private let lock = NSLock()
    private var embeddings: [EmbeddingEntry] = []
    private var etag: String?

    init(endpoint: String, session: URLSession) {
        self.endpoint = endpoint
        self.session = session
    }

    // MARK: - Thread-safe accessors

    private func currentEtag() -> String? {
        lock.lock()
        defer { lock.unlock() }
        return etag
    }

    private func updateCache(entries: [EmbeddingEntry], newEtag: String?) {
        lock.lock()
        defer { lock.unlock() }
        embeddings = entries
        if let newEtag {
            etag = newEtag
        }
    }

    // MARK: - Sync

    /// Fetches advertiser embeddings from the server.
    /// Uses If-None-Match / ETag for 304 caching so repeated calls are cheap.
    func sync() async throws {
        guard let url = URL(string: "\(endpoint)/embeddings") else {
            throw CloudXError.invalidURL("\(endpoint)/embeddings")
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"

        if let existingEtag = currentEtag() {
            request.setValue(existingEtag, forHTTPHeaderField: "If-None-Match")
        }

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else { return }

        if httpResponse.statusCode == 304 {
            return // cache is fresh
        }

        guard httpResponse.statusCode == 200 else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw CloudXError.httpError(statusCode: httpResponse.statusCode, body: body)
        }

        let decoded = try JSONDecoder().decode(EmbeddingsResponse.self, from: data)
        let newEtag = httpResponse.value(forHTTPHeaderField: "ETag")
        updateCache(entries: decoded.embeddings, newEtag: newEtag)
    }

    // MARK: - Proximity

    /// Computes squared Euclidean distance from `queryEmbedding` to each cached
    /// embedding, returning results sorted by distance ascending.
    ///
    /// Uses the Accelerate framework (vDSP) for fast vectorized math.
    private func cachedEmbeddings() -> [EmbeddingEntry] {
        lock.lock()
        defer { lock.unlock() }
        return embeddings
    }

    func proximity(queryEmbedding: [Float]) -> [ProximityResult] {
        let cached = cachedEmbeddings()

        guard !cached.isEmpty else { return [] }

        var results: [ProximityResult] = []
        results.reserveCapacity(cached.count)

        for entry in cached {
            let dist = squaredEuclideanDistance(queryEmbedding, entry.embedding)
            results.append(ProximityResult(id: entry.id, distance: dist))
        }

        results.sort { $0.distance < $1.distance }
        return results
    }
}

// MARK: - Accelerate-backed distance

/// Computes ||a - b||^2 using vDSP for vectorized subtraction and dot product.
func squaredEuclideanDistance(_ a: [Float], _ b: [Float]) -> Float {
    let count = min(a.count, b.count)
    guard count > 0 else { return 0 }

    var diff = [Float](repeating: 0, count: count)
    // diff = a - b
    vDSP_vsub(b, 1, a, 1, &diff, 1, vDSP_Length(count))

    // result = dot(diff, diff) = sum of squares
    var result: Float = 0
    vDSP_dotpr(diff, 1, diff, 1, &result, vDSP_Length(count))

    return result
}
