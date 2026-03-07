package dev.cloudx.sdk

/**
 * A single advertiser embedding entry from the server.
 */
data class EmbeddingEntry(
    val id: String,
    val embedding: FloatArray
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is EmbeddingEntry) return false
        return id == other.id && embedding.contentEquals(other.embedding)
    }

    override fun hashCode(): Int {
        var result = id.hashCode()
        result = 31 * result + embedding.contentHashCode()
        return result
    }
}

/**
 * Result of a proximity calculation: advertiser ID and squared Euclidean distance.
 * Results are sorted ascending by distance (closest first).
 */
data class ProximityResult(
    val id: String,
    val distance: Float
)

/**
 * Bidder information within an ad response.
 */
data class Bidder(
    val id: String,
    val bid: Double,
    val embedding: List<Double>?
)

/**
 * Response from a POST /ad-request call.
 */
data class AdResponse(
    val auctionId: Int,
    val intent: String,
    val winner: Bidder,
    val runnerUp: Bidder?,
    val allBidders: List<Bidder>,
    val payment: Double,
    val currency: String,
    val bidCount: Int,
    val eligibleCount: Int
)

/**
 * Request body for event tracking endpoints.
 */
internal data class EventRequest(
    val auctionId: Int,
    val advertiserId: String,
    val userId: String? = null
)
