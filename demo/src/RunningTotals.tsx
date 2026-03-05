import { useEffect, useState } from "react";
import type { Stats } from "./types";
import { API_BASE } from "./data";

export function RunningTotals() {
  const [stats, setStats] = useState<Stats | null>(null);

  useEffect(() => {
    const load = async () => {
      try {
        const resp = await fetch(`${API_BASE}/stats`);
        if (resp.ok) setStats(await resp.json());
      } catch {
        // ignore
      }
    };
    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, []);

  const s = stats ?? { auction_count: 0, total_spend: 0, cloudx_revenue: 0 };

  return (
    <div style={styles.container}>
      <Stat label="Auctions" value={s.auction_count.toString()} />
      <Stat label="Spend" value={`$${s.total_spend.toFixed(2)}`} />
      <Stat label="Revenue" value={`$${s.cloudx_revenue.toFixed(2)}`} />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div style={styles.stat}>
      <div style={styles.statValue}>{value}</div>
      <div style={styles.statLabel}>{label}</div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: "flex",
    gap: "16px",
  },
  stat: {
    textAlign: "center",
  },
  statValue: {
    fontSize: "16px",
    fontWeight: 700,
    fontFamily: "monospace",
    color: "#1e293b",
  },
  statLabel: {
    fontSize: "11px",
    color: "#64748b",
    textTransform: "uppercase",
    letterSpacing: "0.05em",
  },
};
