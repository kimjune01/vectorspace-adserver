package dev.vectorspace.sdk

import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject

/**
 * Manages local caching of advertiser embeddings with ETag-based
 * conditional fetching (If-None-Match / 304 Not Modified).
 */
internal class EmbeddingCache(
    private val client: OkHttpClient,
    private val endpoint: String
) {
    private var embeddings: List<EmbeddingEntry> = emptyList()
    private var etag: String? = null

    /**
     * Fetch embeddings from the server. Uses ETag for 304 caching so
     * repeated calls are cheap when nothing has changed.
     */
    fun sync() {
        val requestBuilder = Request.Builder()
            .url("$endpoint/embeddings")
            .get()

        etag?.let { requestBuilder.header("If-None-Match", it) }

        val response = client.newCall(requestBuilder.build()).execute()
        response.use { resp ->
            if (resp.code == 304) {
                return // cache is fresh
            }

            if (!resp.isSuccessful) {
                throw VectorSpaceException("Failed to fetch embeddings: HTTP ${resp.code}")
            }

            val body = resp.body?.string()
                ?: throw VectorSpaceException("Empty response body from /embeddings")
            val json = JSONObject(body)
            val embeddingsArray = json.getJSONArray("embeddings")

            val entries = mutableListOf<EmbeddingEntry>()
            for (i in 0 until embeddingsArray.length()) {
                val entry = embeddingsArray.getJSONObject(i)
                val id = entry.getString("id")
                val embeddingJson = entry.getJSONArray("embedding")
                val embedding = FloatArray(embeddingJson.length()) { j ->
                    embeddingJson.getDouble(j).toFloat()
                }
                entries.add(EmbeddingEntry(id, embedding))
            }

            embeddings = entries
            resp.header("ETag")?.let { etag = it }
        }
    }

    /**
     * Compute squared Euclidean distance from [queryEmbedding] to each
     * cached embedding. Returns results sorted ascending by distance.
     */
    fun proximity(queryEmbedding: FloatArray): List<ProximityResult> {
        return embeddings.map { entry ->
            ProximityResult(
                id = entry.id,
                distance = squaredEuclidean(queryEmbedding, entry.embedding)
            )
        }.sortedBy { it.distance }
    }

    /**
     * ||a - b||^2
     */
    private fun squaredEuclidean(a: FloatArray, b: FloatArray): Float {
        var sum = 0f
        val len = minOf(a.size, b.size)
        for (i in 0 until len) {
            val diff = a[i] - b[i]
            sum += diff * diff
        }
        return sum
    }
}

/**
 * Exception thrown by the VectorSpace SDK for server or protocol errors.
 */
class VectorSpaceException(message: String, cause: Throwable? = null) : Exception(message, cause)
