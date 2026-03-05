import { useState } from "react";
import "./index.css";
import { ChatPanel } from "./ChatPanel";
import { ProximityDot } from "./ProximityDot";
import { AuctionPanel } from "./AuctionPanel";
import { AdvertiserSidebar } from "./AdvertiserSidebar";
import { RunningTotals } from "./RunningTotals";
import { PrebuiltMenu } from "./PrebuiltMenu";
import { useChat } from "./useChat";
import { useAdvertisers } from "./useAdvertisers";

function App() {
  const chat = useChat();
  const advs = useAdvertisers();
  const [showAuction, setShowAuction] = useState(false);

  return (
    <div style={styles.root}>
      {/* Top bar */}
      <div style={styles.topBar}>
        <div style={styles.topLeft}>
          <h2 style={styles.logo}>CloudX Demo</h2>
          <PrebuiltMenu
            onSelect={(conv) => chat.loadConversation(conv.messages)}
            onReset={chat.reset}
          />
        </div>
        <RunningTotals />
      </div>

      {/* Main content */}
      <div style={styles.main}>
        {/* Chat panel */}
        <div style={styles.chatCol}>
          <ChatPanel
            messages={chat.messages}
            isLoading={chat.isLoading}
            onSend={chat.sendMessage}
          />
          {/* Dot row at bottom of chat */}
          <div style={styles.dotRow}>
            <ProximityDot
              brightness={chat.dotBrightness}
              onClick={() => setShowAuction(true)}
              hasResult={chat.auctionResult !== null}
            />
          </div>
        </div>

        {/* Advertiser sidebar */}
        <div style={styles.sidebarCol}>
          <AdvertiserSidebar
            advertisers={advs.advertisers}
            onUpdate={advs.updateAdvertiser}
            onDelete={advs.deleteAdvertiser}
            onAdd={advs.addAdvertiser}
          />
        </div>
      </div>

      {/* Auction panel overlay */}
      {showAuction && chat.auctionResult && (
        <AuctionPanel
          result={chat.auctionResult}
          onClose={() => setShowAuction(false)}
        />
      )}
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  root: {
    display: "flex",
    flexDirection: "column",
    height: "100vh",
    overflow: "hidden",
  },
  topBar: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    padding: "8px 16px",
    borderBottom: "1px solid #e2e8f0",
    background: "white",
    flexShrink: 0,
  },
  topLeft: {
    display: "flex",
    alignItems: "center",
    gap: "16px",
  },
  logo: {
    margin: 0,
    fontSize: "18px",
    fontWeight: 700,
    color: "#1e293b",
  },
  main: {
    display: "flex",
    flex: 1,
    overflow: "hidden",
  },
  chatCol: {
    flex: 1,
    display: "flex",
    flexDirection: "column",
    borderRight: "1px solid #e2e8f0",
    minWidth: 0,
  },
  dotRow: {
    display: "flex",
    justifyContent: "flex-end",
    padding: "8px 16px",
    borderTop: "1px solid #e2e8f0",
    background: "white",
    minHeight: "44px",
    alignItems: "center",
  },
  sidebarCol: {
    width: "360px",
    flexShrink: 0,
    background: "#fafbfc",
    overflow: "hidden",
  },
};

export default App;
