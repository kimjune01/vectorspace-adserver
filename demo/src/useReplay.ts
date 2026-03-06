import { useEffect, useRef, useState, useCallback } from "react";
import { publisherDemos } from "./demo-queries";
import { API_BASE } from "./data";

export type ReplayPhase =
  | "idle"
  | "no-match"           // off-topic query in flight
  | "no-match-shown"     // response arrived, no dot — UX is clean
  | "proximity"          // relevant query, dot brightening — phase 1
  | "tap"                // user taps dot — auction fires — phase 2
  | "done";

interface ReplayControls {
  sendChatOnly: (content: string) => void;
  runAuction: () => Promise<void>;
  reset: () => void;
  setShowAuction: (show: boolean) => void;
  setReplayBrightness: (brightness: number | null) => void;
}

/**
 * Demonstrates the two-phase ad lifecycle with multi-turn deepening:
 *
 * 1. Off-topic message → no dot → "your UX stays clean"
 * 2. Multi-turn conversation → dot brightens gradually → user taps → ad shown
 *
 * Dot brightness is faked per step to demonstrate "browsing the ad space."
 */
export function useReplay(
  isReplay: boolean,
  publisherId: string,
  controls: ReplayControls,
  isLoading: boolean,
  hasAuctionResult: boolean,
  messageCount: number,
) {
  const [phase, setPhase] = useState<ReplayPhase>("idle");
  const resolveLoadingRef = useRef<(() => void) | null>(null);
  const resolveResetRef = useRef<(() => void) | null>(null);
  const ranRef = useRef(false);

  // Keep controls in a ref so the async orchestrator always uses the latest
  // version (avoids stale closure over sendMessage which depends on messages state)
  const controlsRef = useRef(controls);
  controlsRef.current = controls;

  // Resolve "wait for loading to finish" promise
  useEffect(() => {
    if (!isLoading && resolveLoadingRef.current) {
      const resolve = resolveLoadingRef.current;
      resolveLoadingRef.current = null;
      resolve();
    }
  }, [isLoading]);

  // Resolve "wait for reset" promise when messages drop to 0
  useEffect(() => {
    if (messageCount === 0 && !isLoading && resolveResetRef.current) {
      const resolve = resolveResetRef.current;
      resolveResetRef.current = null;
      resolve();
    }
  }, [messageCount, isLoading]);

  const delay = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

  const waitForResponse = useCallback(() => {
    return new Promise<void>((resolve) => {
      resolveLoadingRef.current = resolve;
    });
  }, []);

  const waitForReset = useCallback(() => {
    return new Promise<void>((resolve) => {
      resolveResetRef.current = resolve;
    });
  }, []);

  // Main orchestrator — runs once
  useEffect(() => {
    if (!isReplay || ranRef.current) return;
    const demo = publisherDemos.find((d) => d.publisherId === publisherId);
    if (!demo) return;
    ranRef.current = true;

    (async () => {
      // Reset revenue counters so demo starts at $0.00
      await fetch(`${API_BASE}/stats`, { method: "DELETE" }).catch(() => {});
      await delay(500);

      // --- Phase: Off-topic (no dot, UX stays clean) ---
      await delay(1500);
      setPhase("no-match");
      controlsRef.current.sendChatOnly(demo.offTopic);
      await waitForResponse();
      setPhase("no-match-shown");
      await delay(4000);

      // --- Reset and transition to multi-turn deepening ---
      controlsRef.current.setReplayBrightness(0);
      const resetDone = waitForReset();
      controlsRef.current.reset();
      await resetDone;
      await delay(1000);

      // --- Phase: Multi-turn deepening (dot brightens per step) ---
      setPhase("proximity");
      const lastIdx = demo.steps.length - 1;

      for (let i = 0; i < demo.steps.length; i++) {
        const step = demo.steps[i];
        const isLast = i === lastIdx;

        // Always use sendChatOnly — no auction until the user "taps"
        controlsRef.current.sendChatOnly(step.message);
        await waitForResponse();

        // Set faked dot brightness
        controlsRef.current.setReplayBrightness(step.brightness);

        if (isLast) {
          // Final step: bright dot glows — deliberate pause before tap
          await delay(5000);

          // The user "taps" — NOW the auction fires and money moves
          setPhase("tap");
          await controlsRef.current.runAuction();
          // Let React process the auctionResult state update before showing the card
          await delay(100);
          controlsRef.current.setShowAuction(true);
          await delay(6000);
          controlsRef.current.setShowAuction(false);
        } else {
          // Intermediate step: pause to let viewer see the dot change
          await delay(3000);
        }
      }

      // --- Done ---
      controlsRef.current.setReplayBrightness(null);
      setPhase("done");
      await delay(2000);
      document.documentElement.setAttribute("data-replay-done", "true");
    })();
  }, [isReplay, publisherId]);

  return phase;
}
