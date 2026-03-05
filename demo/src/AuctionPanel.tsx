import type { AuctionResult } from "./types";

interface AuctionPanelProps {
  result: AuctionResult;
  onClose: () => void;
}

export function AuctionPanel({ result, onClose }: AuctionPanelProps) {
  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.sheet} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <h3 style={styles.title}>Auction Results</h3>
          <button onClick={onClose} style={styles.closeBtn}>
            &times;
          </button>
        </div>

        <div style={styles.intentRow}>
          <span style={styles.label}>Intent:</span>
          <span style={styles.intentText}>{result.intent}</span>
        </div>

        {result.winner && (
          <div style={styles.winnerCard}>
            <div style={styles.winnerBadge}>WINNER</div>
            <div style={styles.winnerName}>{result.winner.name}</div>
            <div style={styles.winnerIntent}>{result.winner.intent}</div>
            <div style={styles.mathGrid}>
              <MathRow label="Score" value={result.winner.score.toFixed(4)} />
              <MathRow
                label="log(bid)"
                value={result.winner.log_bid.toFixed(4)}
              />
              <MathRow
                label="d\u00B2"
                value={result.winner.distance_sq.toFixed(4)}
              />
              <MathRow label="\u03C3" value={result.winner.sigma.toFixed(2)} />
              <MathRow
                label="Bid Price"
                value={`$${result.winner.bid_price.toFixed(2)}`}
              />
              <MathRow
                label="VCG Payment"
                value={`$${result.payment.toFixed(4)}`}
                highlight
              />
            </div>
          </div>
        )}

        <div style={styles.allBidders}>
          <h4 style={styles.sectionTitle}>
            All Bidders ({result.bid_count} bids, {result.eligible_count}{" "}
            eligible)
          </h4>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>#</th>
                <th style={styles.th}>Name</th>
                <th style={styles.th}>Score</th>
                <th style={styles.th}>d&sup2;</th>
                <th style={styles.th}>&sigma;</th>
                <th style={styles.th}>Bid</th>
              </tr>
            </thead>
            <tbody>
              {result.all_bidders.map((b) => (
                <tr
                  key={b.id}
                  style={
                    b.id === result.winner?.id ? styles.winnerRow : undefined
                  }
                >
                  <td style={styles.td}>{b.rank}</td>
                  <td style={styles.td}>{b.name}</td>
                  <td style={styles.tdMono}>{b.score.toFixed(4)}</td>
                  <td style={styles.tdMono}>{b.distance_sq.toFixed(4)}</td>
                  <td style={styles.tdMono}>{b.sigma.toFixed(2)}</td>
                  <td style={styles.tdMono}>${b.bid_price.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function MathRow({
  label,
  value,
  highlight,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <div
      style={{
        ...styles.mathRow,
        ...(highlight ? styles.mathHighlight : {}),
      }}
    >
      <span style={styles.mathLabel}>{label}</span>
      <span style={styles.mathValue}>{value}</span>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: "fixed",
    inset: 0,
    background: "rgba(0,0,0,0.4)",
    display: "flex",
    alignItems: "flex-end",
    justifyContent: "center",
    zIndex: 100,
  },
  sheet: {
    background: "white",
    borderRadius: "16px 16px 0 0",
    width: "100%",
    maxWidth: "700px",
    maxHeight: "80vh",
    overflowY: "auto",
    padding: "20px",
  },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: "16px",
  },
  title: { margin: 0, fontSize: "18px" },
  closeBtn: {
    background: "none",
    border: "none",
    fontSize: "24px",
    cursor: "pointer",
    color: "#64748b",
  },
  intentRow: {
    marginBottom: "16px",
    padding: "8px 12px",
    background: "#f8fafc",
    borderRadius: "8px",
  },
  label: { fontWeight: 600, marginRight: "8px", fontSize: "13px" },
  intentText: { fontSize: "13px", color: "#475569" },
  winnerCard: {
    background: "linear-gradient(135deg, #fef3c7, #fde68a)",
    borderRadius: "12px",
    padding: "16px",
    marginBottom: "16px",
  },
  winnerBadge: {
    fontSize: "11px",
    fontWeight: 700,
    color: "#92400e",
    letterSpacing: "0.05em",
    marginBottom: "4px",
  },
  winnerName: { fontSize: "16px", fontWeight: 700, color: "#78350f" },
  winnerIntent: {
    fontSize: "12px",
    color: "#92400e",
    marginBottom: "12px",
    lineHeight: "1.4",
  },
  mathGrid: {
    display: "grid",
    gridTemplateColumns: "1fr 1fr",
    gap: "4px",
  },
  mathRow: {
    display: "flex",
    justifyContent: "space-between",
    padding: "4px 8px",
    borderRadius: "4px",
    fontSize: "13px",
  },
  mathHighlight: {
    background: "rgba(120, 53, 15, 0.1)",
    fontWeight: 700,
  },
  mathLabel: { color: "#92400e" },
  mathValue: { fontFamily: "monospace", color: "#78350f" },
  allBidders: { marginTop: "8px" },
  sectionTitle: { fontSize: "14px", marginBottom: "8px", color: "#475569" },
  table: {
    width: "100%",
    borderCollapse: "collapse",
    fontSize: "13px",
  },
  th: {
    textAlign: "left",
    padding: "6px 8px",
    borderBottom: "2px solid #e2e8f0",
    color: "#64748b",
    fontSize: "11px",
    fontWeight: 600,
    textTransform: "uppercase",
  },
  td: { padding: "6px 8px", borderBottom: "1px solid #f1f5f9" },
  tdMono: {
    padding: "6px 8px",
    borderBottom: "1px solid #f1f5f9",
    fontFamily: "monospace",
    fontSize: "12px",
  },
  winnerRow: { background: "#fffbeb" },
};
