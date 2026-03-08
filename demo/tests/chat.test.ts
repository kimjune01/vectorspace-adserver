import { describe, it, expect, vi, beforeEach } from "vitest";
import { chatReply } from "../src/chat";

describe("chatReply", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("sends messages to /chat without a system prompt", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ content: "Hello!" }),
    });
    vi.stubGlobal("fetch", mockFetch);

    const messages = [{ role: "user" as const, content: "Hi" }];
    await chatReply(messages);

    expect(mockFetch).toHaveBeenCalledWith("/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messages }),
    });
  });

  it("returns the content from the response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ content: "I can help with that." }),
      }),
    );

    const result = await chatReply([
      { role: "user", content: "Tell me about sleep" },
    ]);
    expect(result).toBe("I can help with that.");
  });

  it("returns fallback message on 503 (no API key)", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 503,
        text: () => Promise.resolve("Anthropic API key not configured"),
      }),
    );

    const result = await chatReply([{ role: "user", content: "Hi" }]);
    expect(result).toMatch(/demo mode/i);
  });

  it("throws on other errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Internal server error"),
      }),
    );

    await expect(
      chatReply([{ role: "user", content: "Hi" }]),
    ).rejects.toThrow();
  });
});
