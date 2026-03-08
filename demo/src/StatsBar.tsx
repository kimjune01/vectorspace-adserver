import type { Stats } from "./useStats";

export function StatsBar({ stats }: { stats: Stats | null }) {
  if (!stats) return null;

  return (
    <div className="flex gap-4 text-xs text-gray-500 font-mono">
      <span>auctions: {stats.auction_count}</span>
      <span>pub rev: ${stats.publisher_revenue.toFixed(2)}</span>
      <span>exchange rev: ${stats.exchange_revenue.toFixed(2)}</span>
    </div>
  );
}
