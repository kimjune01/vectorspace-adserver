import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { getProfile, updateProfile, getMyAuctions, getMyEvents, downloadMyAuctionsCSV, getCreatives, createCreative, updateCreative, deleteCreative } from '../../api';
import { StatCard } from '../../components/StatCard';
import { DataTable } from '../../components/DataTable';
import { Pagination } from '../../components/Pagination';
import type { PortalProfile, AuctionRow, EventStats, Creative } from '../../types';

export function Dashboard() {
  const [params] = useSearchParams();
  const token = params.get('token') || '';

  const [profile, setProfile] = useState<PortalProfile | null>(null);
  const [auctions, setAuctions] = useState<AuctionRow[]>([]);
  const [auctionTotal, setAuctionTotal] = useState(0);
  const [auctionOffset, setAuctionOffset] = useState(0);
  const [events, setEvents] = useState<EventStats | null>(null);
  const [error, setError] = useState('');

  // Creatives state
  const [creatives, setCreatives] = useState<Creative[]>([]);
  const [newTitle, setNewTitle] = useState('');
  const [newSubtitle, setNewSubtitle] = useState('');
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [editSubtitle, setEditSubtitle] = useState('');

  // Edit state
  const [editName, setEditName] = useState('');
  const [editIntent, setEditIntent] = useState('');
  const [editSigma, setEditSigma] = useState(0.5);
  const [editBid, setEditBid] = useState(0);
  const [editBudget, setEditBudget] = useState(0);
  const [editURL, setEditURL] = useState('');

  useEffect(() => {
    if (!token) return;
    getProfile(token)
      .then((p) => {
        setProfile(p);
        setEditName(p.name);
        setEditIntent(p.intent);
        setEditSigma(p.sigma);
        setEditBid(p.bid_price);
        setEditURL(p.url || '');
      })
      .catch((e) => setError(e.message));

    getMyAuctions(token, 10, 0)
      .then((r) => {
        setAuctions(r.auctions ?? []);
        setAuctionTotal(r.total);
      })
      .catch(() => {});

    getMyEvents(token).then(setEvents).catch(() => {});
    getCreatives(token).then(setCreatives).catch(() => {});
  }, [token]);

  useEffect(() => {
    if (!token) return;
    getMyAuctions(token, 10, auctionOffset)
      .then((r) => {
        setAuctions(r.auctions ?? []);
        setAuctionTotal(r.total);
      })
      .catch(() => {});
  }, [token, auctionOffset]);

  if (!token) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-slate-500">Add <code>?token=YOUR_TOKEN</code> to the URL</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-red-500">{error}</p>
      </div>
    );
  }

  if (!profile) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-slate-400">Loading...</p>
      </div>
    );
  }

  const budgetPct = profile.budget_total > 0
    ? ((profile.budget_spent / profile.budget_total) * 100).toFixed(1)
    : '0';

  const handleSave = async () => {
    await updateProfile(token, {
      name: editName,
      intent: editIntent,
      sigma: editSigma,
      bid_price: editBid,
      budget: editBudget > 0 ? editBudget : undefined,
      url: editURL,
    });
    const p = await getProfile(token);
    setProfile(p);
  };

  return (
    <div className="max-w-5xl mx-auto p-6">
      <h1 className="text-2xl font-bold mb-6">Advertiser Dashboard</h1>

      {/* Budget card */}
      <div className="bg-white rounded-lg border border-slate-200 p-5 mb-6">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-slate-600">Budget</span>
          <span className="text-sm text-slate-400">
            ${profile.budget_spent.toFixed(2)} / ${profile.budget_total.toFixed(2)} {profile.currency}
          </span>
        </div>
        <div className="w-full bg-slate-100 rounded-full h-3">
          <div
            className="bg-blue-500 h-3 rounded-full transition-all"
            style={{ width: `${Math.min(100, Number(budgetPct))}%` }}
          />
        </div>
        <p className="text-xs text-slate-400 mt-1">
          {budgetPct}% used &middot; ${profile.budget_remaining.toFixed(2)} remaining
        </p>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard label="Impressions" value={events?.impressions ?? 0} />
        <StatCard label="Clicks" value={events?.clicks ?? 0} />
        <StatCard label="Viewable" value={events?.viewable ?? 0} />
        <StatCard
          label="CTR"
          value={
            events && events.impressions > 0
              ? ((events.clicks / events.impressions) * 100).toFixed(2) + '%'
              : '-'
          }
        />
        <StatCard
          label="CPC"
          value={
            events && events.clicks > 0
              ? '$' + (profile.budget_spent / events.clicks).toFixed(2)
              : '-'
          }
          sub="Charged per click"
        />
        <StatCard label="Auctions Won" value={auctionTotal} />
      </div>

      {/* Profile edit */}
      <div className="bg-white rounded-lg border border-slate-200 p-5 mb-6">
        <h2 className="text-lg font-semibold mb-4">Profile</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <label className="block">
            <span className="text-sm text-slate-600">Name</span>
            <input
              className="mt-1 block w-full rounded border border-slate-300 px-3 py-2 text-sm"
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="text-sm text-slate-600">Intent</span>
            <input
              className="mt-1 block w-full rounded border border-slate-300 px-3 py-2 text-sm"
              value={editIntent}
              onChange={(e) => setEditIntent(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="text-sm text-slate-600">Sigma ({editSigma.toFixed(2)})</span>
            <input
              type="range"
              min="0.1"
              max="2.0"
              step="0.05"
              className="mt-1 block w-full"
              value={editSigma}
              onChange={(e) => setEditSigma(Number(e.target.value))}
            />
          </label>
          <label className="block">
            <span className="text-sm text-slate-600">Bid Price ($)</span>
            <input
              type="number"
              min="0.01"
              step="0.01"
              className="mt-1 block w-full rounded border border-slate-300 px-3 py-2 text-sm"
              value={editBid}
              onChange={(e) => setEditBid(Number(e.target.value))}
            />
          </label>
          <label className="block">
            <span className="text-sm text-slate-600">Top-up Budget ($)</span>
            <input
              type="number"
              min="0"
              step="10"
              className="mt-1 block w-full rounded border border-slate-300 px-3 py-2 text-sm"
              value={editBudget}
              onChange={(e) => setEditBudget(Number(e.target.value))}
            />
          </label>
          <label className="block md:col-span-2">
            <span className="text-sm text-slate-600">Landing URL</span>
            <input
              type="url"
              placeholder="https://example.com"
              className="mt-1 block w-full rounded border border-slate-300 px-3 py-2 text-sm"
              value={editURL}
              onChange={(e) => setEditURL(e.target.value)}
            />
          </label>
        </div>
        <button
          onClick={handleSave}
          className="mt-4 bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700"
        >
          Save Changes
        </button>
      </div>

      {/* Creatives */}
      <div className="bg-white rounded-lg border border-slate-200 p-5 mb-6">
        <h2 className="text-lg font-semibold mb-4">Creatives</h2>

        {creatives.length > 0 && (
          <div className="mb-4 space-y-2">
            {creatives.map((c) => (
              <div key={c.id} className="flex items-center gap-3 border border-slate-100 rounded p-3">
                {editingId === c.id ? (
                  <>
                    <input
                      className="flex-1 border border-slate-300 rounded px-2 py-1 text-sm"
                      value={editTitle}
                      onChange={(e) => setEditTitle(e.target.value)}
                      placeholder="Title"
                    />
                    <input
                      className="flex-1 border border-slate-300 rounded px-2 py-1 text-sm"
                      value={editSubtitle}
                      onChange={(e) => setEditSubtitle(e.target.value)}
                      placeholder="Subtitle"
                    />
                    <button
                      onClick={async () => {
                        await updateCreative(token, c.id, editTitle, editSubtitle);
                        setEditingId(null);
                        setCreatives(await getCreatives(token));
                      }}
                      className="text-sm text-blue-600 hover:underline"
                    >
                      Save
                    </button>
                    <button
                      onClick={() => setEditingId(null)}
                      className="text-sm text-slate-400 hover:underline"
                    >
                      Cancel
                    </button>
                  </>
                ) : (
                  <>
                    <div className="flex-1">
                      <span className="font-medium text-sm">{c.title}</span>
                      {c.subtitle && <span className="text-slate-500 text-sm ml-2">— {c.subtitle}</span>}
                    </div>
                    <button
                      onClick={() => {
                        setEditingId(c.id);
                        setEditTitle(c.title);
                        setEditSubtitle(c.subtitle);
                      }}
                      className="text-sm text-blue-600 hover:underline"
                    >
                      Edit
                    </button>
                    <button
                      onClick={async () => {
                        await deleteCreative(token, c.id);
                        setCreatives(await getCreatives(token));
                      }}
                      className="text-sm text-red-500 hover:underline"
                    >
                      Delete
                    </button>
                  </>
                )}
              </div>
            ))}
          </div>
        )}

        <div className="flex gap-2">
          <input
            className="flex-1 border border-slate-300 rounded px-3 py-2 text-sm"
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            placeholder="Title"
          />
          <input
            className="flex-1 border border-slate-300 rounded px-3 py-2 text-sm"
            value={newSubtitle}
            onChange={(e) => setNewSubtitle(e.target.value)}
            placeholder="Subtitle (optional)"
          />
          <button
            onClick={async () => {
              if (!newTitle.trim()) return;
              await createCreative(token, newTitle.trim(), newSubtitle.trim());
              setNewTitle('');
              setNewSubtitle('');
              setCreatives(await getCreatives(token));
            }}
            disabled={!newTitle.trim()}
            className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50"
          >
            Add Creative
          </button>
        </div>
      </div>

      {/* Auction history */}
      <div className="bg-white rounded-lg border border-slate-200 p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Auction History</h2>
          <button
            onClick={() => downloadMyAuctionsCSV(token)}
            className="bg-slate-100 text-slate-700 px-4 py-2 rounded text-sm hover:bg-slate-200"
          >
            Export CSV
          </button>
        </div>
        <DataTable
          keyField="id"
          columns={[
            { key: 'id', header: 'ID' },
            { key: 'intent', header: 'Intent' },
            { key: 'payment', header: 'Payment', render: (r) => `$${(r as unknown as AuctionRow).payment.toFixed(4)}` },
            { key: 'bid_count', header: 'Bidders' },
            { key: 'created_at', header: 'Time' },
          ]}
          data={auctions as unknown as Record<string, unknown>[]}
        />
        <Pagination
          total={auctionTotal}
          limit={10}
          offset={auctionOffset}
          onChange={setAuctionOffset}
        />
      </div>
    </div>
  );
}
