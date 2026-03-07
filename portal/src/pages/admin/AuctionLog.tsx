import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { getAdminAuctions, downloadAdminAuctionsCSV, adminLogout } from '../../api';
import { DataTable } from '../../components/DataTable';
import { Pagination } from '../../components/Pagination';
import type { AuctionRow } from '../../types';

export function AuctionLog() {
  const navigate = useNavigate();
  const [auctions, setAuctions] = useState<AuctionRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [winner, setWinner] = useState('');
  const [intent, setIntent] = useState('');
  const limit = 20;

  useEffect(() => {
    getAdminAuctions(limit, offset, winner, intent)
      .then((r) => {
        setAuctions(r.auctions ?? []);
        setTotal(r.total);
      })
      .catch(() => {});
  }, [offset, winner, intent]);

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Auction Log</h1>
        <div className="flex gap-3 text-sm items-center">
          <Link to="/admin" className="text-blue-600 hover:underline">Back to Overview</Link>
          <button onClick={() => { adminLogout(); navigate('/admin/login'); }} className="text-slate-500 hover:text-slate-700">Log out</button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-3 mb-4">
        <input
          placeholder="Filter by winner ID..."
          className="rounded border border-slate-300 px-3 py-2 text-sm flex-1"
          value={winner}
          onChange={(e) => { setWinner(e.target.value); setOffset(0); }}
        />
        <input
          placeholder="Search intent..."
          className="rounded border border-slate-300 px-3 py-2 text-sm flex-1"
          value={intent}
          onChange={(e) => { setIntent(e.target.value); setOffset(0); }}
        />
        <button
          onClick={() => downloadAdminAuctionsCSV(winner, intent)}
          className="bg-slate-100 text-slate-700 px-4 py-2 rounded text-sm hover:bg-slate-200"
        >
          Export CSV
        </button>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 p-5">
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
        <Pagination total={total} limit={limit} offset={offset} onChange={setOffset} />
      </div>
    </div>
  );
}
