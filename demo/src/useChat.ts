import { useState, useCallback } from "react";
import type { ChatMessage } from "@vectorspace/sdk";
import { chatReply } from "./chat";

export function useChat() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);

  const sendMessage = useCallback(async (content: string) => {
    const userMsg: ChatMessage = { role: "user", content };
    setMessages((prev) => [...prev, userMsg]);
    setLoading(true);

    try {
      const allMessages = [...messages, userMsg];
      const reply = await chatReply(allMessages);
      const assistantMsg: ChatMessage = { role: "assistant", content: reply };
      setMessages((prev) => [...prev, assistantMsg]);
    } finally {
      setLoading(false);
    }
  }, [messages]);

  const loadConversation = useCallback((msgs: ChatMessage[]) => {
    setMessages([...msgs]);
  }, []);

  const reset = useCallback(() => {
    setMessages([]);
  }, []);

  return { messages, loading, sendMessage, loadConversation, reset };
}
