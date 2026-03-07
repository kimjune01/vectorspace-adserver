import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { getAdminPublishers, adminLogout } from '../../api';
import { DataTable } from '../../components/DataTable';
import type { PublisherInfo } from '../../types';

export function Publishers() {
  const navigate = useNavigate();
  const [publishers, setPublishers] = useState<PublisherInfo[]>([]);

  useEffect(() => {
    getAdminPublishers().then((r) => setPublishers(r ?? [])).catch(() => {});
  }, []);

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Publishers</h1>
        <div className="flex gap-3 text-sm items-center">
          <Link to="/admin" className="text-blue-600 hover:underline">Back to Overview</Link>
          <button onClick={() => { adminLogout(); navigate('/admin/login'); }} className="text-slate-500 hover:text-slate-700">Log out</button>
        </div>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 p-5">
        <DataTable
          keyField="id"
          columns={[
            { key: 'id', header: 'ID' },
            { key: 'name', header: 'Name' },
            { key: 'domain', header: 'Domain' },
            { key: 'created_at', header: 'Created' },
          ]}
          data={publishers as unknown as Record<string, unknown>[]}
        />
      </div>
    </div>
  );
}
