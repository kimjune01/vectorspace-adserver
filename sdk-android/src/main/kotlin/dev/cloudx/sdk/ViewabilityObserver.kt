package dev.vectorspace.sdk

import android.graphics.Rect
import android.os.Handler
import android.os.Looper
import android.view.View
import android.view.ViewTreeObserver
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

/**
 * Observes a View's visibility and fires a viewable event when the view
 * has been at least 50% visible for 1 continuous second (IAB standard).
 *
 * Once fired, the observer detaches itself and stops polling.
 */
internal class ViewabilityObserver(
    private val view: View,
    private val auctionId: Int,
    private val advertiserId: String,
    private val userId: String?,
    private val eventTracker: EventTracker
) {
    private val handler = Handler(Looper.getMainLooper())
    private var visibleSinceMs: Long = 0L
    private var fired = false

    companion object {
        /** Minimum percentage of the view that must be visible. */
        private const val VISIBILITY_THRESHOLD = 0.50
        /** Duration the view must remain visible (milliseconds). */
        private const val REQUIRED_DURATION_MS = 1000L
        /** How often we check visibility (milliseconds). */
        private const val POLL_INTERVAL_MS = 200L
    }

    private val checkRunnable = object : Runnable {
        override fun run() {
            if (fired) return
            if (isViewSufficientlyVisible()) {
                val now = System.currentTimeMillis()
                if (visibleSinceMs == 0L) {
                    visibleSinceMs = now
                }
                if (now - visibleSinceMs >= REQUIRED_DURATION_MS) {
                    fireViewable()
                    return
                }
            } else {
                visibleSinceMs = 0L
            }
            handler.postDelayed(this, POLL_INTERVAL_MS)
        }
    }

    private val attachListener = object : View.OnAttachStateChangeListener {
        override fun onViewAttachedToWindow(v: View) {
            startPolling()
        }

        override fun onViewDetachedFromWindow(v: View) {
            stopPolling()
        }
    }

    /**
     * Start observing the view for viewability.
     */
    fun observe() {
        view.addOnAttachStateChangeListener(attachListener)
        if (view.isAttachedToWindow) {
            startPolling()
        }
    }

    private fun startPolling() {
        if (fired) return
        handler.postDelayed(checkRunnable, POLL_INTERVAL_MS)
    }

    private fun stopPolling() {
        handler.removeCallbacks(checkRunnable)
        visibleSinceMs = 0L
    }

    private fun fireViewable() {
        fired = true
        stopPolling()
        view.removeOnAttachStateChangeListener(attachListener)

        CoroutineScope(Dispatchers.IO).launch {
            try {
                eventTracker.reportViewable(auctionId, advertiserId, userId)
            } catch (_: Exception) {
                // Best-effort: swallow failures for viewability events
            }
        }
    }

    /**
     * Returns true if at least [VISIBILITY_THRESHOLD] of the view's area
     * is visible on screen and the view itself is shown.
     */
    private fun isViewSufficientlyVisible(): Boolean {
        if (!view.isShown) return false
        if (view.width == 0 || view.height == 0) return false

        val visibleRect = Rect()
        val isVisible = view.getGlobalVisibleRect(visibleRect)
        if (!isVisible) return false

        val visibleArea = visibleRect.width().toLong() * visibleRect.height().toLong()
        val totalArea = view.width.toLong() * view.height.toLong()
        val visibleFraction = visibleArea.toDouble() / totalArea.toDouble()

        return visibleFraction >= VISIBILITY_THRESHOLD
    }
}
