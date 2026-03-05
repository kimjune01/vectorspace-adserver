import { useState, useRef, useEffect } from "react";
import type { ChatMessage } from "./types";

interface ChatPanelProps {
  messages: ChatMessage[];
  isLoading: boolean;
  onSend: (message: string) => void;
}

export function ChatPanel({ messages, isLoading, onSend }: ChatPanelProps) {
  const [input, setInput] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

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
    <div style={styles.container}>
      <div style={styles.messages}>
        {messages.length === 0 && (
          <div style={styles.empty}>
            Start a conversation or pick a prebuilt scenario above.
          </div>
        )}
        {messages.map((msg, i) => (
          <div
            key={i}
            style={{
              ...styles.bubble,
              ...(msg.role === "user" ? styles.userBubble : styles.botBubble),
            }}
          >
            <div style={styles.role}>
              {msg.role === "user" ? "You" : "Assistant"}
            </div>
            <div>{msg.content}</div>
          </div>
        ))}
        {isLoading && (
          <div style={{ ...styles.bubble, ...styles.botBubble }}>
            <div style={styles.typing}>Thinking...</div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>
      <form onSubmit={handleSubmit} style={styles.inputRow}>
        <input
          style={styles.input}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Type a message..."
          disabled={isLoading}
        />
        <button style={styles.sendBtn} type="submit" disabled={isLoading || !input.trim()}>
          Send
        </button>
      </form>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: "flex",
    flexDirection: "column",
    height: "100%",
  },
  messages: {
    flex: 1,
    overflowY: "auto",
    padding: "16px",
    display: "flex",
    flexDirection: "column",
    gap: "8px",
  },
  empty: {
    color: "#888",
    textAlign: "center",
    marginTop: "40px",
    fontSize: "14px",
  },
  bubble: {
    padding: "10px 14px",
    borderRadius: "12px",
    maxWidth: "80%",
    fontSize: "14px",
    lineHeight: "1.5",
  },
  userBubble: {
    alignSelf: "flex-end",
    background: "#2563eb",
    color: "white",
  },
  botBubble: {
    alignSelf: "flex-start",
    background: "#f1f5f9",
    color: "#1e293b",
  },
  role: {
    fontSize: "11px",
    fontWeight: 600,
    marginBottom: "2px",
    opacity: 0.7,
  },
  typing: {
    color: "#888",
    fontStyle: "italic",
  },
  inputRow: {
    display: "flex",
    gap: "8px",
    padding: "12px 16px",
    borderTop: "1px solid #e2e8f0",
  },
  input: {
    flex: 1,
    padding: "10px 14px",
    borderRadius: "8px",
    border: "1px solid #cbd5e1",
    fontSize: "14px",
    outline: "none",
  },
  sendBtn: {
    padding: "10px 20px",
    borderRadius: "8px",
    border: "none",
    background: "#2563eb",
    color: "white",
    fontSize: "14px",
    fontWeight: 600,
    cursor: "pointer",
  },
};
