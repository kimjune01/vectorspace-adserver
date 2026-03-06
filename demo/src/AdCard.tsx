import { useState } from "react";
import type { AuctionResult } from "./types";
import { CloudX } from "./cloudx-sdk";
import { API_BASE } from "./data";

interface AdCardProps {
  result: AuctionResult;
  onClose: () => void;
}

const cloudx = new CloudX({ endpoint: API_BASE });

export function AdCard({ result, onClose }: AdCardProps) {
  const [showSummary, setShowSummary] = useState(false);
  const winner = result.winner;
  if (!winner) return null;

  return (
    <div
      className="fixed inset-0 bg-black/30 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-2xl shadow-2xl max-w-md w-full mx-4 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {showSummary ? (
          <AuctionSummary
            result={result}
            onBack={() => setShowSummary(false)}
          />
        ) : (
          <FakeAd
            result={result}
            onShowSummary={() => setShowSummary(true)}
            onClose={onClose}
          />
        )}
      </div>
    </div>
  );
}

function FakeAd({
  result,
  onShowSummary,
  onClose,
}: {
  result: AuctionResult;
  onShowSummary: () => void;
  onClose: () => void;
}) {
  const winner = result.winner!;

  return (
    <>
      <div className="h-1.5 bg-gradient-to-r from-blue-500 to-purple-500" />
      <div className="p-5">
        <div className="text-[11px] text-slate-400 uppercase tracking-wider mb-3">
          Sponsored
        </div>
        <h3 className="text-lg font-bold text-slate-900 mb-2">
          {winner.name}
        </h3>
        <p className="text-sm text-slate-600 leading-relaxed mb-4">
          {winner.intent}
        </p>
        <button
          className="w-full py-2.5 rounded-lg bg-blue-600 text-white text-sm font-semibold cursor-pointer border-none"
          onClick={() => {
            const clickURL = winner.click_url;
            if (clickURL) {
              window.open(clickURL, "_blank", "noopener");
            }
            cloudx
              .reportClick(result.auction_id, winner.id)
              .catch(() => {});
            onClose();
          }}
        >
          Learn More
        </button>
        <button
          className="w-full mt-2 py-2 rounded-lg bg-transparent border border-slate-200 text-slate-400 text-xs cursor-pointer"
          onClick={onShowSummary}
        >
          How this ad was chosen
        </button>
      </div>
    </>
  );
}

function AuctionSummary({
  result,
  onBack,
}: {
  result: AuctionResult;
  onBack: () => void;
}) {
  const winner = result.winner!;
  const exchangeFee = winner.bid_price - result.payment;

  return (
    <div className="p-5">
      {/* Header */}
      <div className="flex items-center gap-2 mb-4">
        <button
          onClick={onBack}
          className="bg-transparent border-none text-slate-400 cursor-pointer text-sm p-0"
        >
          &larr; Back
        </button>
        <h3 className="m-0 text-base font-bold text-slate-800">
          Auction Summary
        </h3>
      </div>

      {/* Revenue breakdown */}
      <div className="bg-emerald-50 rounded-xl p-4 mb-4">
        <div className="text-[11px] font-semibold text-emerald-700 uppercase tracking-wider mb-2">
          Revenue from this impression
        </div>
        <div className="flex justify-between items-baseline mb-1">
          <span className="text-sm text-emerald-800">Your share</span>
          <span className="text-xl font-bold font-mono text-emerald-900">
            ${result.payment.toFixed(2)}
          </span>
        </div>
        <div className="flex justify-between items-baseline">
          <span className="text-xs text-emerald-600">Exchange fee</span>
          <span className="text-xs font-mono text-emerald-600">
            ${exchangeFee.toFixed(2)}
          </span>
        </div>
        <div className="border-t border-emerald-200 mt-2 pt-2 flex justify-between items-baseline">
          <span className="text-xs text-emerald-600">Advertiser paid</span>
          <span className="text-sm font-mono font-semibold text-emerald-800">
            ${winner.bid_price.toFixed(2)}
          </span>
        </div>
      </div>

      {/* How the match was made */}
      <div className="bg-slate-50 rounded-xl p-4 mb-4">
        <div className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider mb-3">
          How the match was made
        </div>
        <div className="mb-3">
          <div className="text-[11px] text-slate-400 uppercase tracking-wider mb-1">
            Detected need
          </div>
          <div className="text-sm text-slate-700 leading-snug">
            {result.intent}
          </div>
        </div>
        <div>
          <div className="text-[11px] text-slate-400 uppercase tracking-wider mb-1">
            Advertiser position
          </div>
          <div className="text-sm text-slate-700 leading-snug">
            {winner.intent}
          </div>
        </div>
      </div>

      {/* Competition stats */}
      <div className="text-xs text-slate-400 text-center">
        {result.eligible_count} of {result.bid_count} advertisers competed
      </div>
    </div>
  );
}
