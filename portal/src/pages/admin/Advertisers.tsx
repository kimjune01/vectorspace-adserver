import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { getAdminAdvertisers, adminLogout } from '../../api';
import { DataTable } from '../../components/DataTable';
import type { AdvertiserWithBudget } from '../../types';

export function Advertisers() {
  const navigate = useNavigate();
  const [advertisers, setAdvertisers] = useState<AdvertiserWithBudget[]>([]);

  useEffect(() => {
    getAdminAdvertisers().then((r) => setAdvertisers(r ?? [])).catch(() => {});
  }, []);

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Advertisers</h1>
        <div className="flex gap-3 text-sm items-center">
          <Link to="/admin" className="text-blue-600 hover:underline">Back to Overview</Link>
          <button onClick={() => { adminLogout(); navigate('/admin/login'); }} className="text-slate-500 hover:text-slate-700">Log out</button>
        </div>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 p-5">
        <DataTable
          keyField="id"
          columns={[
            { key: 'name', header: 'Name' },
            { key: 'bid_price', header: 'Bid ($)', render: (r) => `$${(r as unknown as AdvertiserWithBudget).bid_price.toFixed(2)}` },
            { key: 'sigma', header: 'Sigma', render: (r) => (r as unknown as AdvertiserWithBudget).sigma.toFixed(2) },
            { key: 'budget_total', header: 'Budget', render: (r) => `$${(r as unknown as AdvertiserWithBudget).budget_total.toFixed(2)}` },
            { key: 'budget_spent', header: 'Spent', render: (r) => `$${(r as unknown as AdvertiserWithBudget).budget_spent.toFixed(2)}` },
            {
              key: 'pct_used',
              header: '% Used',
              render: (r) => {
                const a = r as unknown as AdvertiserWithBudget;
                return a.budget_total > 0 ? `${((a.budget_spent / a.budget_total) * 100).toFixed(1)}%` : '-';
              },
            },
          ]}
          data={advertisers as unknown as Record<string, unknown>[]}
        />
      </div>
    </div>
  );
}
