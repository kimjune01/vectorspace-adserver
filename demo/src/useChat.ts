import { useState, useCallback } from "react";
import type { ChatMessage, AuctionResult } from "./types";
import { API_BASE } from "./data";
import { CloudX } from "./cloudx-sdk";

const cloudx = new CloudX({ endpoint: API_BASE });

function scoreToBrightness(score: number): number {
  return Math.min(1, Math.max(0, (score + 2) / 4));
}

export function useChat() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [auctionResult, setAuctionResult] = useState<AuctionResult | null>(
    null
  );
  const [dotBrightness, setDotBrightness] = useState(0);

  const callChat = async (msgs: ChatMessage[]): Promise<string> => {
    const resp = await fetch(`${API_BASE}/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messages: msgs }),
    });
    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      if (resp.status === 503) {
        throw new Error(
          "Chat is unavailable — the server has no Anthropic API key configured. Start the server with -anthropic-key or set ANTHROPIC_API_KEY."
        );
      }
      throw new Error(text || `Chat API error: ${resp.status}`);
    }
    const data = await resp.json();
    return data.content;
  };

  const runAdFromChat = async (msgs: ChatMessage[]) => {
    const result = await cloudx.requestAdFromChat(msgs);
    if (result) {
      setAuctionResult(result as AuctionResult);
      setDotBrightness(
        result.winner ? scoreToBrightness(result.winner.score) : 0
      );
    } else {
      clearAuction();
    }
  };

  const clearAuction = () => {
    setAuctionResult(null);
    setDotBrightness(0);
  };

  const sendMessage = useCallback(
    async (content: string) => {
      const userMsg: ChatMessage = { role: "user", content };
      const newMessages = [...messages, userMsg];
      setMessages(newMessages);
      setIsLoading(true);

      try {
        const [botContent] = await Promise.all([
          callChat(newMessages),
          runAdFromChat(newMessages).catch(() => clearAuction()),
        ]);

        const botMsg: ChatMessage = { role: "assistant", content: botContent };
        setMessages((prev) => [...prev, botMsg]);
      } catch (err) {
        const errorMsg: ChatMessage = {
          role: "assistant",
          content: `Sorry, an error occurred: ${err instanceof Error ? err.message : "Unknown error"}`,
        };
        setMessages((prev) => [...prev, errorMsg]);
      } finally {
        setIsLoading(false);
      }
    },
    [messages]
  );

  const loadConversation = useCallback(async (msgs: ChatMessage[]) => {
    setMessages(msgs);
    setIsLoading(true);

    try {
      await runAdFromChat(msgs);
    } catch {
      // SDK unavailable — fall back to last user message as raw intent
      const lastUserMsg = [...msgs].reverse().find((m) => m.role === "user");
      if (lastUserMsg) {
        try {
          await cloudx
            .requestAd({ intent: lastUserMsg.content })
            .then((result) => {
              if (result) {
                setAuctionResult(result as AuctionResult);
                setDotBrightness(
                  result.winner ? scoreToBrightness(result.winner.score) : 0
                );
              } else {
                clearAuction();
              }
            });
        } catch {
          clearAuction();
        }
      } else {
        clearAuction();
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  const reset = useCallback(() => {
    setMessages([]);
    clearAuction();
    setIsLoading(false);
  }, []);

  return {
    messages,
    isLoading,
    auctionResult,
    dotBrightness,
    sendMessage,
    loadConversation,
    reset,
  };
}
