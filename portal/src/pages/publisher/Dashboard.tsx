import { useEffect, useState } from 'react';
import { useSearchParams, Navigate, useNavigate } from 'react-router-dom';
import { LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import {
  getPublisherProfile,
  getPublisherStats,
  getPublisherRevenue,
  getPublisherEvents,
  getPublisherAuctions,
  getPublisherTopAdvertisers,
} from '../../api';
import { StatCard } from '../../components/StatCard';
import { DataTable } from '../../components/DataTable';
import { Pagination } from '../../components/Pagination';
import type { PublisherProfile, PublisherStats, RevenuePeriod, AuctionRow, EventStats, AdvertiserSpend } from '../../types';

export function Dashboard() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const token = params.get('token') || localStorage.getItem('publisher_token') || '';

  const [profile, setProfile] = useState<PublisherProfile | null>(null);
  const [stats, setStats] = useState<PublisherStats | null>(null);
  const [revenue, setRevenue] = useState<RevenuePeriod[]>([]);
  const [events, setEvents] = useState<EventStats | null>(null);
  const [auctions, setAuctions] = useState<AuctionRow[]>([]);
  const [auctionTotal, setAuctionTotal] = useState(0);
  const [auctionOffset, setAuctionOffset] = useState(0);
  const [topAdvs, setTopAdvs] = useState<AdvertiserSpend[]>([]);
  const [groupBy, setGroupBy] = useState<'day' | 'week' | 'month'>('day');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!token) return;
    getPublisherProfile(token).then(setProfile).catch((e) => setError(e.message));
    getPublisherStats(token).then(setStats).catch(() => {});
    getPublisherEvents(token).then(setEvents).catch(() => {});
    getPublisherTopAdvertisers(token, 10).then((r) => setTopAdvs(r ?? [])).catch(() => {});
  }, [token]);

  useEffect(() => {
    if (!token) return;
    getPublisherRevenue(token, groupBy).then((r) => setRevenue((r.periods ?? []).reverse())).catch(() => {});
  }, [token, groupBy]);

  useEffect(() => {
    if (!token) return;
    getPublisherAuctions(token, 20, auctionOffset)
      .then((r) => {
        setAuctions(r.auctions ?? []);
        setAuctionTotal(r.total);
      })
      .catch(() => {});
  }, [token, auctionOffset]);

  if (!token) {
    return <Navigate to="/publisher/login" replace />;
  }

  const handleLogout = () => {
    localStorage.removeItem('publisher_token');
    navigate('/publisher/login');
  };

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

  const ctr = events && events.impressions > 0
    ? ((events.clicks / events.impressions) * 100).toFixed(2) + '%'
    : '-';

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Publisher Dashboard</h1>
          <p className="text-sm text-slate-500 mt-1">
            {profile.name}{profile.domain ? ` \u2014 ${profile.domain}` : ''}
          </p>
        </div>
        <button
          onClick={handleLogout}
          className="px-4 py-2 text-sm text-slate-600 border border-slate-300 rounded-md hover:bg-slate-50"
        >
          Log out
        </button>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard
          label="Revenue Earned"
          value={`$${(stats?.total_revenue ?? 0).toFixed(2)}`}
          sub="85% of click payments"
        />
        <StatCard label="Total Auctions" value={stats?.auction_count ?? 0} />
        <StatCard label="Impressions" value={events?.impressions ?? 0} sub={`${events?.clicks ?? 0} clicks`} />
        <StatCard label="CTR" value={ctr} />
        <StatCard
          label="Rev / Click"
          value={
            events && events.clicks > 0
              ? '$' + ((stats?.total_revenue ?? 0) / events.clicks).toFixed(2)
              : '-'
          }
          sub="Your cut per click"
        />
        <StatCard
          label="RPM"
          value={
            events && events.impressions > 0
              ? '$' + (((stats?.total_revenue ?? 0) / events.impressions) * 1000).toFixed(2)
              : '-'
          }
          sub="Revenue per 1k impressions"
        />
        <StatCard label="Clicks" value={events?.clicks ?? 0} />
        <StatCard label="Viewable" value={events?.viewable ?? 0} />
      </div>

      {/* Revenue chart */}
      <div className="bg-white rounded-lg border border-slate-200 p-5 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Revenue</h2>
          <div className="flex gap-1">
            {(['day', 'week', 'month'] as const).map((g) => (
              <button
                key={g}
                onClick={() => setGroupBy(g)}
                className={`px-3 py-1 text-xs rounded ${
                  groupBy === g ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-600'
                }`}
              >
                {g}
              </button>
            ))}
          </div>
        </div>
        <ResponsiveContainer width="100%" height={280}>
          <LineChart data={revenue}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="period" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} />
            <Tooltip />
            <Line type="monotone" dataKey="publisher_revenue" stroke="#10b981" name="Your Revenue" />
            <Line type="monotone" dataKey="total_spend" stroke="#3b82f6" name="Total Spend" />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Top advertisers bar chart */}
      <div className="bg-white rounded-lg border border-slate-200 p-5 mb-6">
        <h2 className="text-lg font-semibold mb-4">Top Advertisers on Your Property</h2>
        {topAdvs.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={topAdvs} layout="vertical">
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis type="number" tick={{ fontSize: 11 }} />
              <YAxis type="category" dataKey="name" tick={{ fontSize: 11 }} width={120} />
              <Tooltip />
              <Bar dataKey="total_spend" fill="#10b981" name="Spend ($)" />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-slate-400 text-sm">No auction data yet</p>
        )}
      </div>

      {/* Auction history */}
      <div className="bg-white rounded-lg border border-slate-200 p-5">
        <h2 className="text-lg font-semibold mb-4">Auction History</h2>
        <DataTable
          keyField="id"
          columns={[
            { key: 'id', header: 'ID' },
            { key: 'intent', header: 'Intent' },
            { key: 'winner_name', header: 'Winner', render: (r) => {
              const a = r as unknown as AuctionRow;
              return a.winner_name || a.winner_id;
            }},
            { key: 'payment', header: 'Payment', render: (r) => `$${(r as unknown as AuctionRow).payment.toFixed(4)}` },
            { key: 'bid_count', header: 'Bidders' },
            { key: 'created_at', header: 'Time' },
          ]}
          data={auctions as unknown as Record<string, unknown>[]}
        />
        <Pagination
          total={auctionTotal}
          limit={20}
          offset={auctionOffset}
          onChange={setAuctionOffset}
        />
      </div>
    </div>
  );
}
