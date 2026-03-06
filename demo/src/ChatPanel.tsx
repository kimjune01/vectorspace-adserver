import { useState, useRef, useEffect, type ReactNode } from "react";
import Markdown from "react-markdown";
import type { ChatMessage } from "./types";
import { useTheme } from "./ThemeContext";

interface ChatPanelProps {
  messages: ChatMessage[];
  isLoading: boolean;
  onSend: (message: string) => void;
  onReset?: () => void;
  dotSlot?: (expanded: boolean) => ReactNode;
}

export function ChatPanel({ messages, isLoading, onSend, onReset, dotSlot }: ChatPanelProps) {
  const [input, setInput] = useState("");
  const [selectedMsg, setSelectedMsg] = useState<number | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const theme = useTheme();

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isLoading]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;
    onSend(input.trim());
    setInput("");
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-2">
        {messages.length === 0 && (
          <div className="text-[var(--theme-text-muted)] text-center mt-10 text-sm">
            {theme.greeting}
          </div>
        )}
        {messages.map((msg, i) => {
          const isLastAssistant =
            msg.role === "assistant" &&
            !messages.slice(i + 1).some((m) => m.role === "assistant");
          const isSelected = selectedMsg === i;
          const isUser = msg.role === "user";
          return (
            <div
              key={i}
              className={isLastAssistant ? "flex items-end gap-2" : undefined}
            >
              <div
                onClick={isLastAssistant ? () => setSelectedMsg(isSelected ? null : i) : undefined}
                className={[
                  "px-3.5 py-2.5 rounded-xl max-w-[80%] text-sm leading-relaxed",
                  isUser
                    ? "self-end bg-[var(--theme-user-bubble)] text-[var(--theme-user-bubble-text)]"
                    : "self-start bg-[var(--theme-bot-bubble)] text-[var(--theme-bot-bubble-text)]",
                  isLastAssistant ? "cursor-pointer" : "",
                  isLastAssistant && isSelected ? "outline-2 outline-amber-400/50" : "",
                ].join(" ")}
              >
                <div className="text-[11px] font-semibold mb-0.5 opacity-70">
                  {isUser ? "You" : "Assistant"}
                </div>
                <Markdown>{msg.content}</Markdown>
              </div>
              {isLastAssistant && dotSlot && (
                <div className="shrink-0 pb-2.5">{dotSlot(isSelected)}</div>
              )}
            </div>
          );
        })}
        {isLoading && (
          <div className="px-3.5 py-2.5 rounded-xl max-w-[80%] text-sm leading-relaxed self-start bg-[var(--theme-bot-bubble)] text-[var(--theme-bot-bubble-text)]">
            <div className="text-[var(--theme-text-muted)] italic">Thinking...</div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>
      <form onSubmit={handleSubmit} className="flex gap-2 px-4 py-3 border-t border-[var(--theme-border)]">
        {onReset && messages.length > 0 && (
          <button
            type="button"
            className="px-2.5 py-2 rounded-lg border border-slate-300 bg-white text-slate-500 text-sm cursor-pointer shrink-0"
            onClick={onReset}
            title="Clear chat"
          >
            ✕
          </button>
        )}
        <input
          className="flex-1 px-3.5 py-2.5 rounded-lg border border-slate-300 text-sm outline-none"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Type a message..."
          disabled={isLoading}
        />
        <button
          className="px-5 py-2.5 rounded-lg border-none bg-[var(--theme-primary)] text-white text-sm font-semibold cursor-pointer"
          type="submit"
          disabled={isLoading || !input.trim()}
        >
          Send
        </button>
      </form>
    </div>
  );
}
