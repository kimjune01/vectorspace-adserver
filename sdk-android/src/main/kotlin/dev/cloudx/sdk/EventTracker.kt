package dev.cloudx.sdk

import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

/**
 * Handles impression, click, and viewable event reporting to the CloudX server.
 */
internal class EventTracker(
    private val client: OkHttpClient,
    private val endpoint: String
) {
    private val jsonMediaType = "application/json; charset=utf-8".toMediaType()

    /**
     * Report an impression event.
     * Returns true if the server accepted it, false if frequency-capped (429).
     */
    fun reportImpression(auctionId: Int, advertiserId: String, userId: String?): Boolean {
        val response = postEvent("/event/impression", auctionId, advertiserId, userId)
        return response != 429
    }

    /**
     * Report a click event.
     */
    fun reportClick(auctionId: Int, advertiserId: String, userId: String?) {
        postEvent("/event/click", auctionId, advertiserId, userId)
    }

    /**
     * Report a viewable event.
     */
    fun reportViewable(auctionId: Int, advertiserId: String, userId: String?) {
        postEvent("/event/viewable", auctionId, advertiserId, userId)
    }

    /**
     * POST an event to the given path and return the HTTP status code.
     */
    private fun postEvent(
        path: String,
        auctionId: Int,
        advertiserId: String,
        userId: String?
    ): Int {
        val json = JSONObject().apply {
            put("auction_id", auctionId)
            put("advertiser_id", advertiserId)
            if (userId != null) {
                put("user_id", userId)
            }
        }

        val requestBody = json.toString().toRequestBody(jsonMediaType)
        val request = Request.Builder()
            .url("$endpoint$path")
            .post(requestBody)
            .build()

        val response = client.newCall(request).execute()
        response.use { resp ->
            if (resp.code == 429) {
                return 429
            }
            if (!resp.isSuccessful) {
                throw CloudXException("Event $path failed: HTTP ${resp.code}")
            }
            return resp.code
        }
    }
}
