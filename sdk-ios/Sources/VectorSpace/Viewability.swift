#if canImport(UIKit)
import UIKit

/// Observes a UIView's on-screen visibility and fires a viewable event
/// once the view has been at least 50% visible for 1 continuous second.
///
/// Based on the IAB viewability standard: 50% of pixels in-view for >= 1 second.
public final class ViewabilityObserver {
    private let view: UIView
    private let auctionId: Int
    private let advertiserId: String
    private let userId: String?
    private let tracker: EventTracker

    private var displayLink: CADisplayLink?
    private var visibleStartTime: CFTimeInterval?
    private var hasFired = false

    /// Minimum fraction of the view that must be visible (0.5 = 50%).
    private let threshold: CGFloat = 0.5
    /// Duration the view must be continuously visible (1 second).
    private let requiredDuration: CFTimeInterval = 1.0

    init(
        view: UIView,
        auctionId: Int,
        advertiserId: String,
        userId: String?,
        tracker: EventTracker
    ) {
        self.view = view
        self.auctionId = auctionId
        self.advertiserId = advertiserId
        self.userId = userId
        self.tracker = tracker
    }

    /// Starts observing. Call this after the view is added to a window.
    func start() {
        guard displayLink == nil, !hasFired else { return }
        let link = CADisplayLink(target: self, selector: #selector(tick))
        link.add(to: .main, forMode: .common)
        displayLink = link
    }

    /// Stops observing and cleans up the display link.
    func stop() {
        displayLink?.invalidate()
        displayLink = nil
        visibleStartTime = nil
    }

    @objc private func tick(_ link: CADisplayLink) {
        guard !hasFired else {
            stop()
            return
        }

        if isViewSufficientlyVisible() {
            if visibleStartTime == nil {
                visibleStartTime = link.timestamp
            }
            let elapsed = link.timestamp - (visibleStartTime ?? link.timestamp)
            if elapsed >= requiredDuration {
                hasFired = true
                stop()
                fireViewable()
            }
        } else {
            // Reset timer -- visibility was interrupted
            visibleStartTime = nil
        }
    }

    /// Checks whether at least `threshold` (50%) of the view is visible on screen.
    private func isViewSufficientlyVisible() -> Bool {
        guard let window = view.window, !view.isHidden, view.alpha > 0 else {
            return false
        }

        let viewRect = view.convert(view.bounds, to: nil)
        let screenRect = window.bounds
        let intersection = viewRect.intersection(screenRect)

        guard !intersection.isNull else { return false }

        let viewArea = viewRect.width * viewRect.height
        guard viewArea > 0 else { return false }

        let visibleArea = intersection.width * intersection.height
        let fraction = visibleArea / viewArea

        return fraction >= threshold
    }

    private func fireViewable() {
        let auctionId = self.auctionId
        let advertiserId = self.advertiserId
        let userId = self.userId
        let tracker = self.tracker

        Task {
            try? await tracker.reportViewable(
                auctionId: auctionId,
                advertiserId: advertiserId,
                userId: userId
            )
        }
    }

    deinit {
        stop()
    }
}
#endif
