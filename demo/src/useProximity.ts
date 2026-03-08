import { useState, useEffect, useRef, useCallback } from "react";
import { VectorSpace } from "@vectorspace/sdk";
import type { ChatMessage, AdResponse } from "@vectorspace/sdk";

export function useProximity(publisherId: string) {
  const vsRef = useRef<VectorSpace | null>(null);
  const [brightness, setBrightness] = useState(0);
  const [ad, setAd] = useState<AdResponse | null>(null);
  const [intent, setIntent] = useState<string>("NONE");
  const [auctionLoading, setAuctionLoading] = useState(false);

  // Create SDK instance on mount / publisherId change
  useEffect(() => {
    const vs = new VectorSpace({
      endpoint: window.location.origin,
      publisherId,
    });
    vsRef.current = vs;
    vs.syncEmbeddings();
  }, [publisherId]);

  const processMessages = useCallback(async (messages: ChatMessage[]) => {
    const vs = vsRef.current;
    if (!vs || messages.length === 0) {
      setBrightness(0);
      setIntent("NONE");
      return;
    }

    try {
      const extracted = await vs.extractIntent(messages);
      setIntent(extracted);

      if (extracted === "NONE") {
        setBrightness(0);
        return;
      }

      // Compute local proximity for dot brightness
      const embedding = await vs.embed(extracted);
      const results = vs.proximity(embedding);

      if (results.length === 0) {
        setBrightness(0);
        return;
      }

      // Convert closest distance to brightness (0-1)
      // distance is squared Euclidean; lower = closer = brighter
      const closest = results[0].distance;
      // Map: distance 0 → brightness 1, distance >= 2 → brightness 0
      const b = Math.max(0, Math.min(1, 1 - closest / 2));
      setBrightness(b);
    } catch {
      // If intent extraction fails (e.g. no API key), keep brightness at 0
      setBrightness(0);
    }
  }, []);

  const requestAuction = useCallback(async () => {
    const vs = vsRef.current;
    if (!vs || intent === "NONE") return;

    setAuctionLoading(true);
    try {
      const result = await vs.requestAd({ intent });
      setAd(result);

      if (result?.winner) {
        await vs.reportImpression(result.auction_id, result.winner.id);
      }
    } finally {
      setAuctionLoading(false);
    }
  }, [intent]);

  const handleClick = useCallback(async () => {
    const vs = vsRef.current;
    if (!vs || !ad?.winner) return;
    await vs.reportClick(ad.auction_id, ad.winner.id);
  }, [ad]);

  const dismissAd = useCallback(() => {
    setAd(null);
  }, []);

  return {
    brightness,
    ad,
    intent,
    auctionLoading,
    processMessages,
    requestAuction,
    handleClick,
    dismissAd,
  };
}
