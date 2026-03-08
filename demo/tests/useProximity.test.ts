import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useProximity } from "../src/useProximity";

// Mock the SDK
const mockSyncEmbeddings = vi.fn().mockResolvedValue(undefined);
const mockExtractIntent = vi.fn();
const mockEmbed = vi.fn();
const mockProximity = vi.fn();
const mockRequestAd = vi.fn();
const mockReportImpression = vi.fn().mockResolvedValue(true);
const mockReportClick = vi.fn().mockResolvedValue(undefined);

vi.mock("@vectorspace/sdk", () => ({
  VectorSpace: vi.fn().mockImplementation(() => ({
    syncEmbeddings: mockSyncEmbeddings,
    extractIntent: mockExtractIntent,
    embed: mockEmbed,
    proximity: mockProximity,
    requestAd: mockRequestAd,
    reportImpression: mockReportImpression,
    reportClick: mockReportClick,
  })),
}));

describe("useProximity", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockExtractIntent.mockResolvedValue("NONE");
    mockEmbed.mockResolvedValue([0.1, 0.2, 0.3]);
    mockProximity.mockReturnValue([{ id: "adv-1", distance: 0.5 }]);
    mockRequestAd.mockResolvedValue(null);
  });

  it("starts with brightness 0 and no ad", () => {
    const { result } = renderHook(() => useProximity("pub-1"));
    expect(result.current.brightness).toBe(0);
    expect(result.current.ad).toBeNull();
  });

  it("syncs embeddings on mount", () => {
    renderHook(() => useProximity("pub-1"));
    expect(mockSyncEmbeddings).toHaveBeenCalled();
  });

  it("updates brightness after processing messages", async () => {
    mockExtractIntent.mockResolvedValue(
      "Therapist helping with anxiety and sleep issues",
    );
    mockProximity.mockReturnValue([{ id: "adv-1", distance: 0.2 }]);

    const { result } = renderHook(() => useProximity("pub-1"));

    const messages = [
      { role: "user" as const, content: "I have anxiety" },
      { role: "assistant" as const, content: "I can help with that" },
    ];

    await act(async () => {
      await result.current.processMessages(messages);
    });

    expect(result.current.brightness).toBeGreaterThan(0);
  });

  it("brightness stays 0 when intent is NONE", async () => {
    mockExtractIntent.mockResolvedValue("NONE");

    const { result } = renderHook(() => useProximity("pub-1"));

    await act(async () => {
      await result.current.processMessages([
        { role: "user", content: "What's a good recipe?" },
        { role: "assistant", content: "Try pasta!" },
      ]);
    });

    expect(result.current.brightness).toBe(0);
  });

  it("requestAuction fires ad request and returns result", async () => {
    const mockAd = {
      auction_id: 1,
      intent: "therapy",
      winner: { id: "adv-1", name: "BetterHelp", ad_title: "Get Help", ad_subtitle: "Talk to a therapist", bid_price: 20 },
      payment: 12.5,
      currency: "USD",
      bid_count: 5,
      eligible_count: 3,
    };
    mockRequestAd.mockResolvedValue(mockAd);
    mockExtractIntent.mockResolvedValue("therapy");

    const { result } = renderHook(() => useProximity("pub-1"));

    // Set intent first
    await act(async () => {
      await result.current.processMessages([
        { role: "user", content: "I need therapy" },
        { role: "assistant", content: "I can help" },
      ]);
    });

    await act(async () => {
      await result.current.requestAuction();
    });

    expect(result.current.ad).toEqual(mockAd);
    expect(mockReportImpression).toHaveBeenCalledWith(1, "adv-1");
  });

  it("dismissAd clears the ad", async () => {
    const mockAd = {
      auction_id: 1,
      intent: "therapy",
      winner: { id: "adv-1", name: "BetterHelp" },
      payment: 12.5,
      currency: "USD",
      bid_count: 5,
      eligible_count: 3,
    };
    mockRequestAd.mockResolvedValue(mockAd);
    mockExtractIntent.mockResolvedValue("therapy");

    const { result } = renderHook(() => useProximity("pub-1"));

    await act(async () => {
      await result.current.processMessages([
        { role: "user", content: "I need therapy" },
        { role: "assistant", content: "I can help" },
      ]);
    });

    await act(async () => {
      await result.current.requestAuction();
    });
    expect(result.current.ad).not.toBeNull();

    act(() => {
      result.current.dismissAd();
    });
    expect(result.current.ad).toBeNull();
  });
});
