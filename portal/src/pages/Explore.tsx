import { useState } from 'react';
import { Link } from 'react-router-dom';
import { simulateAuction } from '../api';
import { StatCard } from '../components/StatCard';
import type { SimulationResult } from '../types';

function distanceColor(distSq: number): string {
  if (distSq <= 0.5) return 'bg-green-50';
  if (distSq <= 2.0) return 'bg-yellow-50';
  return 'bg-red-50';
}

export function Explore() {
  const [intent, setIntent] = useState('');
  const [result, setResult] = useState<SimulationResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function handleSimulate() {
    const trimmed = intent.trim();
    if (!trimmed) return;
    setLoading(true);
    setError('');
    try {
      const res = await simulateAuction(trimmed);
      setResult(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Simulation failed');
      setResult(null);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="flex items-center justify-between mb-2">
        <h1 className="text-2xl font-bold">Position Explorer</h1>
        <Link to="/admin" className="text-sm text-blue-600 hover:underline">Admin Dashboard</Link>
      </div>
      <p className="text-slate-500 text-sm mb-6">
        Enter a user intent to simulate an auction and see how advertisers rank.
      </p>

      <div className="flex gap-3 mb-6">
        <input
          type="text"
          value={intent}
          onChange={(e) => setIntent(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSimulate()}
          placeholder="e.g. best running shoes for flat feet"
          className="flex-1 border border-slate-300 rounded-lg px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <button
          onClick={handleSimulate}
          disabled={loading || !intent.trim()}
          className="px-5 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? 'Simulating...' : 'Simulate'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm mb-4">{error}</p>}

      {result && (
        <>
          {/* Summary stat cards */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <StatCard label="Total Advertisers" value={result.bid_count} />
            <StatCard
              label="Eligible Bidders"
              value={result.all_bidders.length}
            />
            <StatCard
              label="VCG Payment"
              value={`$${result.payment.toFixed(4)}`}
            />
            <StatCard
              label="Winner"
              value={result.winner?.name ?? 'None'}
              sub={result.winner ? `Score: ${result.winner.score.toFixed(4)}` : undefined}
            />
          </div>

          {/* Tau threshold analysis */}
          <div className="bg-white rounded-lg border border-slate-200 p-5 mb-6">
            <h2 className="text-lg font-semibold mb-3">Tau Threshold Analysis</h2>
            <p className="text-xs text-slate-400 mb-3">
              How many advertisers pass at each relevance threshold
            </p>
            <div className="flex flex-wrap gap-2">
              {result.tau_thresholds.map((t) => (
                <span
                  key={t.tau}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm border border-slate-200 bg-slate-50"
                >
                  <span className="text-slate-500">tau={t.tau}</span>
                  <span className="font-semibold">{t.count}</span>
                </span>
              ))}
            </div>
          </div>

          {/* Results table */}
          <div className="bg-white rounded-lg border border-slate-200 overflow-hidden">
            <div className="p-5 border-b border-slate-200">
              <h2 className="text-lg font-semibold">All Bidders</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-slate-50 text-left text-slate-500">
                    <th className="px-4 py-3 font-medium">Rank</th>
                    <th className="px-4 py-3 font-medium">Name</th>
                    <th className="px-4 py-3 font-medium">Intent</th>
                    <th className="px-4 py-3 font-medium text-right">Bid</th>
                    <th className="px-4 py-3 font-medium text-right">Sigma</th>
                    <th className="px-4 py-3 font-medium text-right">Score</th>
                    <th className="px-4 py-3 font-medium text-right">Distance²</th>
                  </tr>
                </thead>
                <tbody>
                  {result.all_bidders.map((b) => (
                    <tr key={b.id} className={`border-t border-slate-100 ${distanceColor(b.distance_sq)}`}>
                      <td className="px-4 py-3 font-mono">{b.rank}</td>
                      <td className="px-4 py-3 font-medium">{b.name}</td>
                      <td className="px-4 py-3 text-slate-600 truncate max-w-xs">{b.intent}</td>
                      <td className="px-4 py-3 text-right font-mono">${b.bid_price.toFixed(2)}</td>
                      <td className="px-4 py-3 text-right font-mono">{b.sigma.toFixed(2)}</td>
                      <td className="px-4 py-3 text-right font-mono">{b.score.toFixed(4)}</td>
                      <td className="px-4 py-3 text-right font-mono">{b.distance_sq.toFixed(4)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
