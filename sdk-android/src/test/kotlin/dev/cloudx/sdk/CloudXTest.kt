package dev.cloudx.sdk

import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import kotlinx.coroutines.runBlocking
import org.json.JSONArray
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test

class CloudXTest {

    private lateinit var server: MockWebServer
    private lateinit var cloudx: CloudX

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        cloudx = CloudX(server.url("/").toString())
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    // ── Proximity / distance tests (pure logic, no server) ───────────

    @Test
    fun `proximity returns empty list when cache is empty`() {
        val results = cloudx.proximity(floatArrayOf(0.5f, 0.5f, 0.5f))
        assertTrue(results.isEmpty())
    }

    @Test
    fun `proximity sorts by ascending distance`() = runBlocking {
        val embeddingsJson = JSONObject().apply {
            put("version", "v1")
            put("embeddings", JSONArray().apply {
                put(JSONObject().apply {
                    put("id", "adv-1")
                    put("embedding", JSONArray(listOf(0.1, 0.2, 0.3)))
                })
                put(JSONObject().apply {
                    put("id", "adv-2")
                    put("embedding", JSONArray(listOf(0.9, 0.8, 0.7)))
                })
            })
        }

        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setHeader("ETag", "\"v1\"")
                .setBody(embeddingsJson.toString())
        )

        cloudx.syncEmbeddings()

        // Query near adv-1
        val results = cloudx.proximity(floatArrayOf(0.1f, 0.2f, 0.3f))
        assertEquals(2, results.size)
        assertEquals("adv-1", results[0].id)
        assertEquals(0.0f, results[0].distance, 0.0001f)
        assertEquals("adv-2", results[1].id)
        // (0.9-0.1)^2 + (0.8-0.2)^2 + (0.7-0.3)^2 = 0.64 + 0.36 + 0.16 = 1.16
        assertEquals(1.16f, results[1].distance, 0.01f)
    }

    @Test
    fun `proximity closest changes with query`() = runBlocking {
        val embeddingsJson = JSONObject().apply {
            put("version", "v1")
            put("embeddings", JSONArray().apply {
                put(JSONObject().apply {
                    put("id", "adv-1")
                    put("embedding", JSONArray(listOf(0.1, 0.2, 0.3)))
                })
                put(JSONObject().apply {
                    put("id", "adv-2")
                    put("embedding", JSONArray(listOf(0.9, 0.8, 0.7)))
                })
            })
        }

        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setHeader("ETag", "\"v1\"")
                .setBody(embeddingsJson.toString())
        )

        cloudx.syncEmbeddings()

        // Query near adv-2
        val results = cloudx.proximity(floatArrayOf(0.9f, 0.8f, 0.7f))
        assertEquals("adv-2", results[0].id)
        assertEquals(0.0f, results[0].distance, 0.0001f)
    }

    // ── syncEmbeddings / ETag caching ────────────────────────────────

    @Test
    fun `syncEmbeddings fetches on first call`() = runBlocking {
        val embeddingsJson = JSONObject().apply {
            put("version", "v1")
            put("embeddings", JSONArray().apply {
                put(JSONObject().apply {
                    put("id", "adv-1")
                    put("embedding", JSONArray(listOf(0.5, 0.5, 0.5)))
                })
            })
        }

        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setHeader("ETag", "\"v1\"")
                .setBody(embeddingsJson.toString())
        )

        cloudx.syncEmbeddings()

        val results = cloudx.proximity(floatArrayOf(0.5f, 0.5f, 0.5f))
        assertEquals(1, results.size)
        assertEquals("adv-1", results[0].id)

        // Verify request was made without If-None-Match
        val request = server.takeRequest()
        assertNull(request.getHeader("If-None-Match"))
    }

    @Test
    fun `syncEmbeddings uses ETag for 304 caching`() = runBlocking {
        val embeddingsJson = JSONObject().apply {
            put("version", "v1")
            put("embeddings", JSONArray().apply {
                put(JSONObject().apply {
                    put("id", "adv-1")
                    put("embedding", JSONArray(listOf(0.5, 0.5, 0.5)))
                })
            })
        }

        // First call: 200 with ETag
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setHeader("ETag", "\"v1\"")
                .setBody(embeddingsJson.toString())
        )

        // Second call: 304
        server.enqueue(MockResponse().setResponseCode(304))

        cloudx.syncEmbeddings()
        cloudx.syncEmbeddings()

        // Cache should still be intact after 304
        val results = cloudx.proximity(floatArrayOf(0.5f, 0.5f, 0.5f))
        assertEquals(1, results.size)

        // Verify second request included If-None-Match
        server.takeRequest() // skip first
        val secondRequest = server.takeRequest()
        assertEquals("\"v1\"", secondRequest.getHeader("If-None-Match"))
    }

    // ── requestAd ────────────────────────────────────────────────────

    @Test
    fun `requestAd returns AdResponse on success`() = runBlocking {
        val responseJson = JSONObject().apply {
            put("auction_id", 42)
            put("intent", "back pain treatment")
            put("winner", JSONObject().apply {
                put("id", "adv-1")
                put("bid", 2.50)
            })
            put("runner_up", JSONObject().apply {
                put("id", "adv-2")
                put("bid", 1.80)
            })
            put("all_bidders", JSONArray().apply {
                put(JSONObject().apply {
                    put("id", "adv-1")
                    put("bid", 2.50)
                })
                put(JSONObject().apply {
                    put("id", "adv-2")
                    put("bid", 1.80)
                })
            })
            put("payment", 1.81)
            put("currency", "USD")
            put("bid_count", 2)
            put("eligible_count", 2)
        }

        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(responseJson.toString())
        )

        val ad = cloudx.requestAd("back pain treatment")

        assertNotNull(ad)
        assertEquals(42, ad!!.auctionId)
        assertEquals("back pain treatment", ad.intent)
        assertEquals("adv-1", ad.winner.id)
        assertEquals(2.50, ad.winner.bid, 0.001)
        assertNotNull(ad.runnerUp)
        assertEquals("adv-2", ad.runnerUp!!.id)
        assertEquals(1.81, ad.payment, 0.001)
        assertEquals("USD", ad.currency)
        assertEquals(2, ad.bidCount)
        assertEquals(2, ad.eligibleCount)

        // Verify the request body
        val request = server.takeRequest()
        assertEquals("POST", request.method)
        val requestBody = JSONObject(request.body.readUtf8())
        assertEquals("back pain treatment", requestBody.getString("intent"))
    }

    @Test
    fun `requestAd sends tau when provided`() = runBlocking {
        val responseJson = JSONObject().apply {
            put("auction_id", 1)
            put("intent", "test")
            put("winner", JSONObject().apply {
                put("id", "adv-1")
                put("bid", 1.0)
            })
            put("all_bidders", JSONArray().apply {
                put(JSONObject().apply {
                    put("id", "adv-1")
                    put("bid", 1.0)
                })
            })
            put("payment", 0.5)
            put("currency", "USD")
            put("bid_count", 1)
            put("eligible_count", 1)
        }

        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(responseJson.toString())
        )

        cloudx.requestAd("test", tau = 0.5)

        val request = server.takeRequest()
        val requestBody = JSONObject(request.body.readUtf8())
        assertEquals(0.5, requestBody.getDouble("tau"), 0.001)
    }

    @Test
    fun `requestAd returns null when no bidders`() = runBlocking {
        server.enqueue(
            MockResponse()
                .setResponseCode(500)
                .setBody("no bidders passed the eligibility threshold")
        )

        val ad = cloudx.requestAd("obscure intent no one bids on")
        assertNull(ad)
    }

    // ── reportImpression ─────────────────────────────────────────────

    @Test
    fun `reportImpression returns true on success`() = runBlocking {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody("""{"status":"ok"}""")
        )

        val result = cloudx.reportImpression(42, "adv-1")
        assertTrue(result)

        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertTrue(request.path!!.contains("/event/impression"))
        val body = JSONObject(request.body.readUtf8())
        assertEquals(42, body.getInt("auction_id"))
        assertEquals("adv-1", body.getString("advertiser_id"))
        assertFalse(body.has("user_id"))
    }

    @Test
    fun `reportImpression includes userId when provided`() = runBlocking {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody("""{"status":"ok"}""")
        )

        cloudx.reportImpression(42, "adv-1", userId = "user-123")

        val request = server.takeRequest()
        val body = JSONObject(request.body.readUtf8())
        assertEquals("user-123", body.getString("user_id"))
    }

    @Test
    fun `reportImpression returns false on 429`() = runBlocking {
        server.enqueue(
            MockResponse()
                .setResponseCode(429)
                .setBody("frequency cap exceeded")
        )

        val result = cloudx.reportImpression(42, "adv-1", userId = "user-123")
        assertFalse(result)
    }

    // ── reportClick ──────────────────────────────────────────────────

    @Test
    fun `reportClick sends correct payload`() = runBlocking {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody("""{"status":"ok"}""")
        )

        cloudx.reportClick(42, "adv-1", userId = "user-456")

        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertTrue(request.path!!.contains("/event/click"))
        val body = JSONObject(request.body.readUtf8())
        assertEquals(42, body.getInt("auction_id"))
        assertEquals("adv-1", body.getString("advertiser_id"))
        assertEquals("user-456", body.getString("user_id"))
    }

    // ── reportViewable ───────────────────────────────────────────────

    @Test
    fun `reportViewable sends correct payload`() = runBlocking {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody("""{"status":"ok"}""")
        )

        cloudx.reportViewable(42, "adv-1")

        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertTrue(request.path!!.contains("/event/viewable"))
        val body = JSONObject(request.body.readUtf8())
        assertEquals(42, body.getInt("auction_id"))
        assertEquals("adv-1", body.getString("advertiser_id"))
    }

    // ── Model tests ──────────────────────────────────────────────────

    @Test
    fun `ProximityResult data class equality`() {
        val a = ProximityResult("adv-1", 0.5f)
        val b = ProximityResult("adv-1", 0.5f)
        assertEquals(a, b)
    }

    @Test
    fun `EmbeddingEntry equality uses contentEquals`() {
        val a = EmbeddingEntry("adv-1", floatArrayOf(0.1f, 0.2f, 0.3f))
        val b = EmbeddingEntry("adv-1", floatArrayOf(0.1f, 0.2f, 0.3f))
        assertEquals(a, b)
        assertEquals(a.hashCode(), b.hashCode())
    }

    @Test
    fun `EmbeddingEntry inequality on different embeddings`() {
        val a = EmbeddingEntry("adv-1", floatArrayOf(0.1f, 0.2f, 0.3f))
        val b = EmbeddingEntry("adv-1", floatArrayOf(0.9f, 0.8f, 0.7f))
        assertNotEquals(a, b)
    }

    @Test
    fun `AdResponse stores all fields`() {
        val winner = Bidder("adv-1", 2.50, null)
        val runnerUp = Bidder("adv-2", 1.80, null)
        val ad = AdResponse(
            auctionId = 42,
            intent = "test",
            winner = winner,
            runnerUp = runnerUp,
            allBidders = listOf(winner, runnerUp),
            payment = 1.81,
            currency = "USD",
            bidCount = 2,
            eligibleCount = 2
        )

        assertEquals(42, ad.auctionId)
        assertEquals("test", ad.intent)
        assertEquals("adv-1", ad.winner.id)
        assertEquals("adv-2", ad.runnerUp?.id)
        assertEquals(2, ad.allBidders.size)
        assertEquals(1.81, ad.payment, 0.001)
        assertEquals("USD", ad.currency)
    }
}
