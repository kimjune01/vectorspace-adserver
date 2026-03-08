/**
 * @vectorspace/sdk/viewability — Browser-only entry point.
 *
 * Provides IAB-standard viewability observation (50%+ visible for 1 second).
 * Separate entry point so Node.js users don't pull in DOM types.
 */

import { EventTracker } from "./event-tracker.js";

export interface ViewabilityOptions {
  /** The ad exchange endpoint. */
  endpoint: string;
  /** Publisher ID for event attribution. */
  publisherId?: string;
}

/**
 * Observe an HTML element for IAB viewability (50%+ visible for 1+ second).
 * Fires reportViewable automatically when the threshold is met.
 *
 * Returns a cleanup function that disconnects the observer.
 */
export function observeViewability(
  el: HTMLElement,
  auctionId: number,
  advertiserId: string,
  options: ViewabilityOptions,
  userId?: string,
): () => void {
  const tracker = new EventTracker(options.endpoint, options.publisherId);
  let timer: ReturnType<typeof setTimeout> | null = null;
  let fired = false;

  const observer = new IntersectionObserver(
    (entries) => {
      const entry = entries[0];
      if (fired) return;

      if (entry.intersectionRatio >= 0.5) {
        if (!timer) {
          timer = setTimeout(() => {
            fired = true;
            tracker.reportViewable(auctionId, advertiserId, userId);
            observer.disconnect();
          }, 1000);
        }
      } else {
        if (timer) {
          clearTimeout(timer);
          timer = null;
        }
      }
    },
    { threshold: [0, 0.5] },
  );

  observer.observe(el);

  return () => {
    if (timer) clearTimeout(timer);
    observer.disconnect();
  };
}
