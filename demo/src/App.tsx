import { useState, useEffect, useCallback, useRef } from "react";
import { ThemeProvider, useTheme } from "./ThemeProvider";
import { ChatPanel } from "./ChatPanel";
import { ProximityDot } from "./ProximityDot";
import { AdCard } from "./AdCard";
import { StatsBar } from "./StatsBar";
import { ConversationPicker } from "./ConversationPicker";
import { conversations } from "./conversations";
import { useChat } from "./useChat";
import { useProximity } from "./useProximity";
import { useStats } from "./useStats";

function DemoInner() {
  const theme = useTheme();
  const chat = useChat();
  const proximity = useProximity(theme.publisherId);
  const stats = useStats();

  // Only process for proximity after a new assistant reply lands
  const lastProcessed = useRef(0);
  useEffect(() => {
    const msgs = chat.messages;
    if (
      msgs.length >= 2 &&
      msgs.length > lastProcessed.current &&
      msgs[msgs.length - 1].role === "assistant"
    ) {
      lastProcessed.current = msgs.length;
      proximity.processMessages(msgs);
    }
  }, [chat.messages.length]);

  const handleSelectConversation = useCallback(
    (index: number) => {
      const c = conversations[index];
      proximity.dismissAd();
      chat.loadConversation(c.messages);
    },
    [chat, proximity],
  );

  const handleReset = useCallback(() => {
    chat.reset();
    proximity.dismissAd();
  }, [chat, proximity]);

  return (
    <div
      className="flex flex-col h-dvh"
      style={{ backgroundColor: "var(--color-bg)" }}
    >
      {/* Header */}
      <header className="border-b bg-white px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1
            className="text-lg font-semibold"
            style={{ color: "var(--color-primary)" }}
          >
            {theme.name}
          </h1>
          <span className="text-xs text-gray-400">
            powered by Vector Space Exchange
          </span>
        </div>
        <StatsBar stats={stats} />
      </header>

      {/* Conversation picker */}
      <div className="px-4 py-2 border-b bg-white/80">
        <ConversationPicker
          onSelect={handleSelectConversation}
          onReset={handleReset}
        />
      </div>

      {/* Chat */}
      <div className="flex-1 overflow-hidden">
        <ChatPanel
          messages={
            chat.messages.length > 0
              ? chat.messages
              : [{ role: "assistant" as const, content: theme.greeting }]
          }
          loading={chat.loading}
          onSend={chat.sendMessage}
          dotSlot={
            <ProximityDot
              brightness={proximity.brightness}
              onClick={proximity.requestAuction}
              loading={proximity.auctionLoading}
            />
          }
        />
      </div>

      {/* Ad overlay */}
      {proximity.ad && proximity.ad.winner && (
        <AdCard
          ad={proximity.ad}
          onClose={proximity.dismissAd}
          onClick={proximity.handleClick}
        />
      )}
    </div>
  );
}

export default function App() {
  const [publisherId, setPublisherId] = useState("pub-1");

  return (
    <ThemeProvider publisherId={publisherId}>
      <DemoInner key={publisherId} />
      {/* Theme toggle — top-right to avoid mobile keyboard overlap */}
      <div className="fixed top-3 right-4 z-40">
        <button
          onClick={() =>
            setPublisherId((p) => (p === "pub-1" ? "pub-2" : "pub-1"))
          }
          className="bg-white shadow-lg rounded-full px-3 py-1.5 text-xs text-gray-600
                     hover:text-gray-900 border cursor-pointer"
        >
          Switch theme
        </button>
      </div>
    </ThemeProvider>
  );
}
