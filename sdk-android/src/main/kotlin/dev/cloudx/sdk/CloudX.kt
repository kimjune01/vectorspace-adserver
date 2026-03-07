package dev.cloudx.sdk

import android.view.View
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.TimeUnit

/**
 * CloudX Android SDK — main client for the CloudX ad server.
 *
 * Usage:
 * ```kotlin
 * val cloudx = CloudX("http://localhost:8080")
 * cloudx.syncEmbeddings()
 * val ad = cloudx.requestAd("back pain from sitting")
 * if (ad != null) {
 *     cloudx.reportImpression(ad.auctionId, ad.winner.id)
 * }
 * ```
 *
 * All network calls are suspending functions that run on [Dispatchers.IO].
 */
class CloudX(private val endpoint: String) {

    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(10, TimeUnit.SECONDS)
        .build()

    private val jsonMediaType = "application/json; charset=utf-8".toMediaType()
    private val embeddingCache = EmbeddingCache(client, endpoint.trimEnd('/'))
    private val eventTracker = EventTracker(client, endpoint.trimEnd('/'))

    private val baseEndpoint: String = endpoint.trimEnd('/')

    // ── Embedding cache ──────────────────────────────────────────────

    /**
     * Fetch advertiser embeddings from the server.
     * Uses ETag / If-None-Match for efficient 304 caching.
     */
    suspend fun syncEmbeddings() {
        withContext(Dispatchers.IO) {
            embeddingCache.sync()
        }
    }

    /**
     * Compute squared Euclidean distance from [queryEmbedding] to each
     * cached advertiser embedding. Returns results sorted ascending by
     * distance (closest advertiser first).
     */
    fun proximity(queryEmbedding: FloatArray): List<ProximityResult> {
        return embeddingCache.proximity(queryEmbedding)
    }

    // ── Ad requests ──────────────────────────────────────────────────

    /**
     * Request an ad for the given [intent].
     *
     * @param intent  The user's intent string.
     * @param tau     Optional temperature parameter for auction softmax.
     * @return The [AdResponse] if a winner was found, or null if no bidders passed.
     */
    suspend fun requestAd(intent: String, tau: Double? = null): AdResponse? {
        return withContext(Dispatchers.IO) {
            val json = JSONObject().apply {
                put("intent", intent)
                if (tau != null && tau > 0) {
                    put("tau", tau)
                }
            }

            val requestBody = json.toString().toRequestBody(jsonMediaType)
            val request = Request.Builder()
                .url("$baseEndpoint/ad-request")
                .post(requestBody)
                .build()

            val response = client.newCall(request).execute()
            response.use { resp ->
                if (resp.code == 500) {
                    val errorBody = resp.body?.string() ?: ""
                    if ("no bidders passed" in errorBody) {
                        return@withContext null
                    }
                    throw CloudXException("Ad request failed: HTTP 500 - $errorBody")
                }

                if (!resp.isSuccessful) {
                    throw CloudXException("Ad request failed: HTTP ${resp.code}")
                }

                val body = resp.body?.string()
                    ?: throw CloudXException("Empty response body from /ad-request")
                parseAdResponse(JSONObject(body))
            }
        }
    }

    // ── Event Tracking ───────────────────────────────────────────────

    /**
     * Report an impression event.
     * Returns false if the server responds with 429 (frequency-capped).
     */
    suspend fun reportImpression(
        auctionId: Int,
        advertiserId: String,
        userId: String? = null
    ): Boolean {
        return withContext(Dispatchers.IO) {
            eventTracker.reportImpression(auctionId, advertiserId, userId)
        }
    }

    /**
     * Report a click event.
     */
    suspend fun reportClick(
        auctionId: Int,
        advertiserId: String,
        userId: String? = null
    ) {
        withContext(Dispatchers.IO) {
            eventTracker.reportClick(auctionId, advertiserId, userId)
        }
    }

    /**
     * Report a viewable event.
     */
    suspend fun reportViewable(
        auctionId: Int,
        advertiserId: String,
        userId: String? = null
    ) {
        withContext(Dispatchers.IO) {
            eventTracker.reportViewable(auctionId, advertiserId, userId)
        }
    }

    // ── View observability ───────────────────────────────────────────

    /**
     * Observe a [View] for IAB viewability: fires [reportViewable] when
     * the view is at least 50% visible for 1 continuous second.
     *
     * Must be called from the main thread.
     */
    fun observeViewability(
        view: View,
        auctionId: Int,
        advertiserId: String,
        userId: String? = null
    ) {
        val observer = ViewabilityObserver(
            view = view,
            auctionId = auctionId,
            advertiserId = advertiserId,
            userId = userId,
            eventTracker = eventTracker
        )
        observer.observe()
    }

    // ── JSON parsing ─────────────────────────────────────────────────

    private fun parseAdResponse(json: JSONObject): AdResponse {
        return AdResponse(
            auctionId = json.getInt("auction_id"),
            intent = json.getString("intent"),
            winner = parseBidder(json.getJSONObject("winner")),
            runnerUp = if (json.has("runner_up") && !json.isNull("runner_up"))
                parseBidder(json.getJSONObject("runner_up")) else null,
            allBidders = parseBidderList(json.getJSONArray("all_bidders")),
            payment = json.getDouble("payment"),
            currency = json.getString("currency"),
            bidCount = json.getInt("bid_count"),
            eligibleCount = json.getInt("eligible_count")
        )
    }

    private fun parseBidder(json: JSONObject): Bidder {
        val embedding = if (json.has("embedding") && !json.isNull("embedding")) {
            val arr = json.getJSONArray("embedding")
            List(arr.length()) { i -> arr.getDouble(i) }
        } else null

        return Bidder(
            id = json.getString("id"),
            bid = json.getDouble("bid"),
            embedding = embedding
        )
    }

    private fun parseBidderList(array: org.json.JSONArray): List<Bidder> {
        return List(array.length()) { i -> parseBidder(array.getJSONObject(i)) }
    }
}
