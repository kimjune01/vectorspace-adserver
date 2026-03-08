import type { AdResponse } from "@vectorspace/sdk";

export function AdCard({
  ad,
  onClose,
  onClick,
}: {
  ad: AdResponse;
  onClose: () => void;
  onClick: () => void;
}) {
  const winner = ad.winner;
  if (!winner) return null;

  const handleCTA = () => {
    onClick();
    if (winner.click_url) {
      window.open(winner.click_url, "_blank", "noopener");
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/30">
      <div
        className="bg-white rounded-t-2xl sm:rounded-2xl w-full sm:max-w-md shadow-xl
                    animate-[slideUp_0.3s_ease-out] p-6 relative"
      >
        {/* Close button */}
        <button
          onClick={onClose}
          className="absolute top-3 right-3 text-gray-400 hover:text-gray-600 text-xl
                     leading-none cursor-pointer"
        >
          &times;
        </button>

        {/* Sponsor label */}
        <div className="text-xs text-gray-400 uppercase tracking-wider mb-3">
          Sponsored
        </div>

        {/* Creative */}
        <h3 className="text-lg font-semibold text-gray-900">
          {winner.ad_title || winner.name}
        </h3>
        {winner.ad_subtitle && (
          <p className="text-sm text-gray-600 mt-1">{winner.ad_subtitle}</p>
        )}

        {/* CTA */}
        <button
          onClick={handleCTA}
          className="mt-4 w-full py-2.5 rounded-xl text-white text-sm font-medium
                     cursor-pointer hover:opacity-90 transition-opacity"
          style={{ backgroundColor: "var(--color-primary)" }}
        >
          Learn More
        </button>

        {/* Auction summary */}
        <details className="mt-4 text-xs text-gray-400">
          <summary className="cursor-pointer hover:text-gray-600">
            Auction details
          </summary>
          <div className="mt-2 space-y-1 font-mono">
            <div>auction #{ad.auction_id}</div>
            <div>
              winner: {winner.name} (${winner.bid_price.toFixed(2)} bid)
            </div>
            <div>payment: ${ad.payment.toFixed(4)}</div>
            <div>
              bidders: {ad.bid_count} total, {ad.eligible_count} eligible
            </div>
            <div>distance: {winner.distance_sq.toFixed(4)}</div>
            {ad.runner_up && (
              <div>runner-up: {ad.runner_up.name}</div>
            )}
          </div>
        </details>
      </div>
    </div>
  );
}
