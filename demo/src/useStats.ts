import { useState, useEffect } from "react";

export interface Stats {
  auction_count: number;
  total_spend: number;
  publisher_revenue: number;
  exchange_revenue: number;
  advertiser_count: number;
  publisher_count: number;
}

export function useStats(intervalMs = 3000) {
  const [stats, setStats] = useState<Stats | null>(null);

  useEffect(() => {
    let active = true;

    const fetchStats = async () => {
      try {
        const resp = await fetch("/stats");
        if (resp.ok && active) {
          setStats(await resp.json());
        }
      } catch {
        // Ignore fetch errors
      }
    };

    fetchStats();
    const id = setInterval(fetchStats, intervalMs);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, [intervalMs]);

  return stats;
}
