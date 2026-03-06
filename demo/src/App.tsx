import { useState, useMemo } from "react";
import "./index.css";
import { ChatPanel } from "./ChatPanel";
import { ProximityDot } from "./ProximityDot";
import { AdCard } from "./AdCard";
import { AdvertiserSidebar } from "./AdvertiserSidebar";
import { RunningTotals } from "./RunningTotals";
import { PrebuiltMenu } from "./PrebuiltMenu";
import { useChat } from "./useChat";
import { useAdvertisers } from "./useAdvertisers";
import { useTheme } from "./ThemeContext";
import { useReplay, type ReplayPhase } from "./useReplay";

function App() {
  const theme = useTheme();
  const chat = useChat(theme.defaultTau);
  const advs = useAdvertisers();
  const [showAuction, setShowAuction] = useState(false);

  const params = useMemo(() => new URLSearchParams(window.location.search), []);
  const isReplay = params.get("replay") === "true";

  const replayPhase = useReplay(
    isReplay,
    theme.id,
    {
      sendChatOnly: chat.sendChatOnly,
      runAuction: chat.runAuction,
      reset: chat.reset,
      setShowAuction,
      setReplayBrightness: chat.setReplayBrightness,
    },
    chat.isLoading,
    chat.auctionResult !== null,
    chat.messages.length,
  );

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-[var(--theme-bg)]">
      {/* Top bar */}
      <div className="flex justify-between items-center px-4 py-2 border-b border-[var(--theme-border)] bg-[var(--theme-surface)] shrink-0">
        <div className="flex items-center gap-4">
          <h2 className="m-0 text-lg font-bold text-[var(--theme-text)]">{theme.name}</h2>
          {!isReplay && (
            <PrebuiltMenu
              onSelect={(conv) => chat.loadConversation(conv.messages)}
              onReset={chat.reset}
            />
          )}
        </div>
        <RunningTotals />
      </div>

      {/* Phase banner for replay */}
      {isReplay && <PhaseBanner phase={replayPhase} />}

      {/* Main content */}
      <div className="flex flex-1 overflow-hidden">
        {/* Chat panel */}
        <div className={`flex-1 flex flex-col min-w-0 ${isReplay ? "" : "border-r border-[var(--theme-border)]"}`}>
          <ChatPanel
            messages={chat.messages}
            isLoading={chat.isLoading}
            onSend={chat.sendMessage}
            onReset={isReplay ? undefined : chat.reset}
            dotSlot={(expanded) => (
              <ProximityDot
                brightness={chat.dotBrightness}
                onClick={() => setShowAuction(true)}
                hasResult={chat.auctionResult !== null}
                expanded={expanded}
              />
            )}
          />
        </div>

        {/* Advertiser sidebar */}
        {!isReplay && (
          <div className="w-[360px] shrink-0 bg-[var(--theme-surface)] overflow-hidden">
            <AdvertiserSidebar
              advertisers={advs.advertisers}
              onUpdate={advs.updateAdvertiser}
              onDelete={advs.deleteAdvertiser}
              onAdd={advs.addAdvertiser}
            />
          </div>
        )}
      </div>

      {/* Ad card overlay */}
      {showAuction && chat.auctionResult && (
        <AdCard
          result={chat.auctionResult}
          onClose={() => setShowAuction(false)}
        />
      )}
    </div>
  );
}

function PhaseBanner({ phase }: { phase: ReplayPhase }) {
  if (phase === "idle" || phase === "done") return null;

  const banners: Record<string, { bg: string; text: string; label: string }> = {
    "no-match": {
      bg: "bg-slate-100", text: "text-slate-500",
      label: "Off-topic conversation — no ad loads, no interruption",
    },
    "no-match-shown": {
      bg: "bg-slate-100", text: "text-slate-600",
      label: "Your user experience stays clean",
    },
    "proximity": {
      bg: "bg-blue-50", text: "text-blue-700",
      label: "A dot reflects nearby expertise — separate from the chatbot. No ad has loaded. No money has moved.",
    },
    "tap": {
      bg: "bg-emerald-50", text: "text-emerald-700",
      label: "The user tapped — auction fired, you earned revenue",
    },
  };

  const b = banners[phase];
  if (!b) return null;

  return (
    <div className={`px-4 py-2 text-center text-sm font-medium border-b border-[var(--theme-border)] ${b.bg} ${b.text}`}>
      {b.label}
    </div>
  );
}

export default App;
