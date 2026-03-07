import Foundation

/// Handles impression, click, and viewable event reporting to the CloudX server.
final class EventTracker: @unchecked Sendable {
    private let endpoint: String
    private let session: URLSession
    private let encoder = JSONEncoder()

    init(endpoint: String, session: URLSession) {
        self.endpoint = endpoint
        self.session = session
    }

    // MARK: - Impression

    /// Reports an impression event.
    /// Returns `true` on success, `false` if the server responds 429 (frequency capped).
    func reportImpression(auctionId: Int, advertiserId: String, userId: String?) async throws -> Bool {
        let body = EventBody(auctionId: auctionId, advertiserId: advertiserId, userId: userId)
        do {
            try await post(path: "/event/impression", body: body)
            return true
        } catch CloudXError.httpError(statusCode: 429, _) {
            return false
        }
    }

    // MARK: - Click

    /// Reports a click event.
    func reportClick(auctionId: Int, advertiserId: String, userId: String?) async throws {
        let body = EventBody(auctionId: auctionId, advertiserId: advertiserId, userId: userId)
        try await post(path: "/event/click", body: body)
    }

    // MARK: - Viewable

    /// Reports a viewable event.
    func reportViewable(auctionId: Int, advertiserId: String, userId: String?) async throws {
        let body = EventBody(auctionId: auctionId, advertiserId: advertiserId, userId: userId)
        try await post(path: "/event/viewable", body: body)
    }

    // MARK: - Private

    private func post<T: Encodable>(path: String, body: T) async throws {
        guard let url = URL(string: "\(endpoint)\(path)") else {
            throw CloudXError.invalidURL("\(endpoint)\(path)")
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try encoder.encode(body)

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else { return }

        guard (200...299).contains(httpResponse.statusCode) else {
            let responseBody = String(data: data, encoding: .utf8) ?? ""
            throw CloudXError.httpError(statusCode: httpResponse.statusCode, body: responseBody)
        }
    }
}
