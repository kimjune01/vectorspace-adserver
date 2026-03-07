import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { getStats, getAdminRevenue, getAdminTopAdvertisers, getAdminEvents, adminLogout } from '../../api';
import { StatCard } from '../../components/StatCard';
import type { RevenuePeriod, AdvertiserSpend, EventStats } from '../../types';

export function Overview() {
  const navigate = useNavigate();
  const [stats, setStats] = useState<{
    auction_count: number;
    total_spend: number;
    publisher_revenue: number;
    exchange_revenue: number;
    advertiser_count: number;
    publisher_count: number;
  } | null>(null);
  const [revenue, setRevenue] = useState<RevenuePeriod[]>([]);
  const [topAdvs, setTopAdvs] = useState<AdvertiserSpend[]>([]);
  const [events, setEvents] = useState<EventStats | null>(null);
  const [groupBy, setGroupBy] = useState<'day' | 'week' | 'month'>('day');

  useEffect(() => {
    getStats().then(setStats).catch(() => {});
    getAdminTopAdvertisers(10).then((r) => setTopAdvs(r ?? [])).catch(() => {});
    getAdminEvents().then(setEvents).catch(() => {});
  }, []);

  useEffect(() => {
    getAdminRevenue(groupBy).then((r) => setRevenue((r.periods ?? []).reverse())).catch(() => {});
  }, [groupBy]);

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Admin Dashboard</h1>
        <div className="flex gap-3 text-sm items-center">
          <Link to="/admin/auctions" className="text-blue-600 hover:underline">Auction Log</Link>
          <Link to="/admin/advertisers" className="text-blue-600 hover:underline">Advertisers</Link>
          <Link to="/admin/publishers" className="text-blue-600 hover:underline">Publishers</Link>
          <button onClick={() => { adminLogout(); navigate('/admin/login'); }} className="text-slate-500 hover:text-slate-700">Log out</button>
        </div>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard label="Total Auctions" value={stats?.auction_count ?? 0} />
        <StatCard label="Total Revenue" value={`$${(stats?.total_spend ?? 0).toFixed(2)}`} />
        <StatCard label="Exchange Cut" value={`$${(stats?.exchange_revenue ?? 0).toFixed(2)}`} sub="15% of revenue" />
        <StatCard label="Impressions" value={events?.impressions ?? 0} sub={`${events?.clicks ?? 0} clicks`} />
        <StatCard label="Advertisers" value={stats?.advertiser_count ?? 0} />
        <StatCard label="Publishers" value={stats?.publisher_count ?? 0} />
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
            <Line type="monotone" dataKey="total_spend" stroke="#3b82f6" name="Total" />
            <Line type="monotone" dataKey="exchange_revenue" stroke="#f97316" name="Exchange" />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Top advertisers bar chart */}
      <div className="bg-white rounded-lg border border-slate-200 p-5">
        <h2 className="text-lg font-semibold mb-4">Top Advertisers by Spend</h2>
        {topAdvs.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={topAdvs} layout="vertical">
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis type="number" tick={{ fontSize: 11 }} />
              <YAxis type="category" dataKey="name" tick={{ fontSize: 11 }} width={120} />
              <Tooltip />
              <Bar dataKey="total_spend" fill="#3b82f6" name="Spend ($)" />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-slate-400 text-sm">No auction data yet</p>
        )}
      </div>
    </div>
  );
}
