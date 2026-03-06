import type { AuctionResult } from "./types";

interface AuctionPanelProps {
  result: AuctionResult;
  onClose: () => void;
}

export function AuctionPanel({ result, onClose }: AuctionPanelProps) {
  return (
    <div
      className="fixed inset-0 bg-black/40 flex items-end justify-center z-100"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-t-2xl w-full max-w-[700px] max-h-[80vh] overflow-y-auto p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-center mb-4">
          <h3 className="m-0 text-lg">Auction Results</h3>
          <button
            onClick={onClose}
            className="bg-transparent border-none text-2xl cursor-pointer text-slate-500"
          >
            &times;
          </button>
        </div>

        <div className="mb-4 px-3 py-2 bg-slate-50 rounded-lg">
          <span className="font-semibold mr-2 text-[13px]">Intent:</span>
          <span className="text-[13px] text-slate-600">{result.intent}</span>
        </div>

        {result.winner && (
          <div className="bg-gradient-to-br from-amber-100 to-amber-200 rounded-xl p-4 mb-4">
            <div className="text-[11px] font-bold text-amber-800 tracking-wide mb-1">
              WINNER
            </div>
            <div className="text-base font-bold text-amber-900">
              {result.winner.name}
            </div>
            <div className="text-xs text-amber-800 mb-3 leading-snug">
              {result.winner.intent}
            </div>
            <div className="grid grid-cols-2 gap-1">
              <MathRow label="Score" value={result.winner.score.toFixed(4)} />
              <MathRow label="log(bid)" value={result.winner.log_bid.toFixed(4)} />
              <MathRow label="d&sup2;" value={result.winner.distance_sq.toFixed(4)} />
              <MathRow label="&sigma;" value={result.winner.sigma.toFixed(2)} />
              <MathRow label="Bid Price" value={`$${result.winner.bid_price.toFixed(2)}`} />
              <MathRow label="VCG Payment" value={`$${result.payment.toFixed(4)}`} highlight />
            </div>
          </div>
        )}

        <div className="mt-2">
          <h4 className="text-sm mb-2 text-slate-600">
            All Bidders ({result.bid_count} bids, {result.eligible_count} eligible)
          </h4>
          <table className="w-full border-collapse text-[13px]">
            <thead>
              <tr>
                <th className="text-left px-2 py-1.5 border-b-2 border-slate-200 text-slate-500 text-[11px] font-semibold uppercase">#</th>
                <th className="text-left px-2 py-1.5 border-b-2 border-slate-200 text-slate-500 text-[11px] font-semibold uppercase">Name</th>
                <th className="text-left px-2 py-1.5 border-b-2 border-slate-200 text-slate-500 text-[11px] font-semibold uppercase">Score</th>
                <th className="text-left px-2 py-1.5 border-b-2 border-slate-200 text-slate-500 text-[11px] font-semibold uppercase">d&sup2;</th>
                <th className="text-left px-2 py-1.5 border-b-2 border-slate-200 text-slate-500 text-[11px] font-semibold uppercase">&sigma;</th>
                <th className="text-left px-2 py-1.5 border-b-2 border-slate-200 text-slate-500 text-[11px] font-semibold uppercase">Bid</th>
              </tr>
            </thead>
            <tbody>
              {result.all_bidders.map((b) => (
                <tr
                  key={b.id}
                  className={b.id === result.winner?.id ? "bg-amber-50" : ""}
                >
                  <td className="px-2 py-1.5 border-b border-slate-100">{b.rank}</td>
                  <td className="px-2 py-1.5 border-b border-slate-100">{b.name}</td>
                  <td className="px-2 py-1.5 border-b border-slate-100 font-mono text-xs">{b.score.toFixed(4)}</td>
                  <td className="px-2 py-1.5 border-b border-slate-100 font-mono text-xs">{b.distance_sq.toFixed(4)}</td>
                  <td className="px-2 py-1.5 border-b border-slate-100 font-mono text-xs">{b.sigma.toFixed(2)}</td>
                  <td className="px-2 py-1.5 border-b border-slate-100 font-mono text-xs">${b.bid_price.toFixed(2)}</td>
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
      className={`flex justify-between px-2 py-1 rounded text-[13px] ${highlight ? "bg-amber-900/10 font-bold" : ""}`}
    >
      <span className="text-amber-800" dangerouslySetInnerHTML={{ __html: label }} />
      <span className="font-mono text-amber-900">{value}</span>
    </div>
  );
}
