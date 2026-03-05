import { StrictMode, useState, useCallback } from "react";
import { createRoot } from "react-dom/client";
import { CloudX, type AdResponse, type AdBidder } from "./cloudx-sdk";
import "./index.css";

const cloudx = new CloudX({ endpoint: "http://localhost:8080" });

function scoreToBrightness(score: number): number {
  return Math.min(1, Math.max(0, (score + 2) / 4));
}

function Probe() {
  const [input, setInput] = useState("");
  const [tau, setTau] = useState(0);
  const [result, setResult] = useState<AdResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const probe = useCallback(async (intent: string, tauVal: number) => {
    if (!intent.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const ad = await cloudx.requestAd({
        intent,
        tau: tauVal > 0 ? tauVal : undefined,
      });
      if (ad) {
        setResult(ad);
      } else {
        setError("No ads passed the relevance gate");
        setResult(null);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed");
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    probe(input, tau);
  };

  const brightness = result?.winner
    ? scoreToBrightness(result.winner.score)
    : 0;

  return (
    <div style={styles.page}>
      <div style={styles.center}>
        <Dot brightness={brightness} hasResult={result !== null} />

        {result?.winner && (
          <WinnerLabel winner={result.winner} payment={result.payment} />
        )}

        <form onSubmit={handleSubmit} style={styles.form}>
          <input
            style={styles.input}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Type an intent to probe..."
            disabled={loading}
            autoFocus
          />
          <button
            style={styles.btn}
            type="submit"
            disabled={loading || !input.trim()}
          >
            {loading ? "..." : "Probe"}
          </button>
        </form>

        <div style={styles.tauRow}>
          <label style={styles.tauLabel}>
            τ {tau === 0 ? "(off)" : tau.toFixed(2)}
          </label>
          <input
            type="range"
            min="0"
            max="2"
            step="0.01"
            value={tau}
            onChange={(e) => setTau(parseFloat(e.target.value))}
            style={styles.tauSlider}
          />
        </div>

        {error && <div style={styles.error}>{error}</div>}

        {result && (
          <div style={styles.table}>
            <div style={styles.meta}>
              {result.bid_count} bidders &middot; payment $
              {result.payment.toFixed(4)}
            </div>
            <table style={styles.tableEl}>
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
                      b.id === result.winner?.id
                        ? styles.winnerRow
                        : undefined
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
        )}
      </div>
    </div>
  );
}

function Dot({
  brightness,
  hasResult,
}: {
  brightness: number;
  hasResult: boolean;
}) {
  const size = 80;
  const amber = hasResult
    ? `rgba(245, 158, 11, ${0.3 + brightness * 0.7})`
    : "rgba(148, 163, 184, 0.3)";
  const glowSize = hasResult ? 10 + brightness * 40 : 0;

  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: "50%",
        background: amber,
        boxShadow: glowSize
          ? `0 0 ${glowSize}px ${glowSize / 2}px ${amber}`
          : "none",
        transition: "all 0.4s ease",
        marginBottom: 24,
      }}
    />
  );
}

function WinnerLabel({
  winner,
  payment,
}: {
  winner: AdBidder;
  payment: number;
}) {
  return (
    <div style={styles.winner}>
      <div style={styles.winnerName}>{winner.name}</div>
      <div style={styles.winnerDetail}>
        score {winner.score.toFixed(4)} &middot; d&sup2;{" "}
        {winner.distance_sq.toFixed(4)} &middot; pays ${payment.toFixed(4)}
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    minHeight: "100vh",
    display: "flex",
    justifyContent: "center",
    alignItems: "flex-start",
    paddingTop: "10vh",
    background: "#f8fafc",
  },
  center: {
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    width: "100%",
    maxWidth: 700,
    padding: "0 20px",
  },
  form: {
    display: "flex",
    gap: 8,
    width: "100%",
    maxWidth: 500,
  },
  input: {
    flex: 1,
    padding: "12px 16px",
    borderRadius: 8,
    border: "1px solid #cbd5e1",
    fontSize: 15,
    outline: "none",
  },
  btn: {
    padding: "12px 24px",
    borderRadius: 8,
    border: "none",
    background: "#2563eb",
    color: "white",
    fontSize: 15,
    fontWeight: 600,
    cursor: "pointer",
  },
  tauRow: {
    display: "flex",
    alignItems: "center",
    gap: 12,
    width: "100%",
    maxWidth: 500,
    marginTop: 12,
  },
  tauLabel: {
    fontSize: 13,
    fontFamily: "monospace",
    color: "#64748b",
    minWidth: 80,
  },
  tauSlider: {
    flex: 1,
  },
  error: {
    color: "#dc2626",
    fontSize: 13,
    marginTop: 12,
  },
  winner: {
    textAlign: "center",
    marginBottom: 16,
  },
  winnerName: {
    fontSize: 18,
    fontWeight: 700,
    color: "#78350f",
  },
  winnerDetail: {
    fontSize: 13,
    color: "#92400e",
    fontFamily: "monospace",
    marginTop: 2,
  },
  table: {
    width: "100%",
    marginTop: 24,
  },
  meta: {
    fontSize: 13,
    color: "#64748b",
    marginBottom: 8,
    textAlign: "center",
  },
  tableEl: {
    width: "100%",
    borderCollapse: "collapse",
    fontSize: 13,
  },
  th: {
    textAlign: "left",
    padding: "6px 8px",
    borderBottom: "2px solid #e2e8f0",
    color: "#64748b",
    fontSize: 11,
    fontWeight: 600,
    textTransform: "uppercase",
  },
  td: { padding: "6px 8px", borderBottom: "1px solid #f1f5f9" },
  tdMono: {
    padding: "6px 8px",
    borderBottom: "1px solid #f1f5f9",
    fontFamily: "monospace",
    fontSize: 12,
  },
  winnerRow: { background: "#fffbeb" },
};

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Probe />
  </StrictMode>
);
