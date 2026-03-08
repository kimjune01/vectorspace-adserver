import { useRef, useEffect, useState, type ReactNode } from "react";
import type { ChatMessage } from "@vectorspace/sdk";

export function ChatPanel({
  messages,
  loading,
  onSend,
  dotSlot,
}: {
  messages: ChatMessage[];
  loading: boolean;
  onSend: (content: string) => void;
  dotSlot?: ReactNode;
}) {
  const [input, setInput] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length, loading]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = input.trim();
    if (!trimmed || loading) return;
    setInput("");
    onSend(trimmed);
  };

  return (
    <div className="flex flex-col h-full">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {messages.map((msg, i) => (
          <div key={i}>
            <div
              className={`max-w-[80%] rounded-2xl px-4 py-2 text-sm leading-relaxed ${
                msg.role === "user"
                  ? "ml-auto text-white"
                  : "mr-auto text-gray-900"
              }`}
              style={{
                backgroundColor:
                  msg.role === "user"
                    ? "var(--color-user-bubble)"
                    : "var(--color-assistant-bubble)",
              }}
            >
              {msg.content}
            </div>
            {/* Dot slot after last assistant message */}
            {msg.role === "assistant" && i === messages.length - 1 && dotSlot && (
              <div className="mt-1.5 ml-1">{dotSlot}</div>
            )}
          </div>
        ))}
        {loading && (
          <div
            className="max-w-[80%] mr-auto rounded-2xl px-4 py-2 text-sm text-gray-400"
            style={{ backgroundColor: "var(--color-assistant-bubble)" }}
          >
            Thinking...
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <form onSubmit={handleSubmit} className="border-t p-3 flex gap-2">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Type a message..."
          disabled={loading}
          className="flex-1 border border-gray-300 rounded-full px-4 py-2 text-sm
                     focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]
                     disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={loading || !input.trim()}
          className="px-4 py-2 rounded-full text-sm text-white font-medium
                     disabled:opacity-50 cursor-pointer disabled:cursor-not-allowed"
          style={{ backgroundColor: "var(--color-primary)" }}
        >
          Send
        </button>
      </form>
    </div>
  );
}
