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

  const s = {
    auction_count: 0,
    total_spend: 0,
    publisher_revenue: 0,
    exchange_revenue: 0,
    ...stats,
  };

  const reset = async () => {
    try {
      await fetch(`${API_BASE}/stats`, { method: "DELETE" });
      setStats({ auction_count: 0, total_spend: 0, publisher_revenue: 0, exchange_revenue: 0 });
    } catch {
      // ignore
    }
  };

  return (
    <div className="flex items-center gap-4">
      <button
        onClick={reset}
        className="bg-transparent border border-slate-300 rounded-md cursor-pointer text-base text-slate-500 px-2 py-1 leading-none"
        title="Reset stats"
      >
        ↺
      </button>
      <Stat label="Auctions" value={s.auction_count.toString()} />
      <Stat label="Publisher" value={`$${s.publisher_revenue.toFixed(2)}`} />
      <Stat label="Exchange" value={`$${s.exchange_revenue.toFixed(2)}`} />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="text-center">
      <div className="text-base font-bold font-mono text-[var(--theme-text)]">{value}</div>
      <div className="text-[11px] text-[var(--theme-text-muted)] uppercase tracking-wide">{label}</div>
    </div>
  );
}
