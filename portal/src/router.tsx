import { Routes, Route, Navigate } from 'react-router-dom';
import { Dashboard } from './pages/advertiser/Dashboard';
import { Dashboard as PublisherDashboard } from './pages/publisher/Dashboard';
import { Login as PublisherLogin } from './pages/publisher/Login';
import { AdminLogin } from './pages/admin/Login';
import { Overview } from './pages/admin/Overview';
import { AuctionLog } from './pages/admin/AuctionLog';
import { Advertisers } from './pages/admin/Advertisers';
import { Publishers } from './pages/admin/Publishers';
import { Explore } from './pages/Explore';
import { NotFound } from './pages/NotFound';
import { getAdminPassword } from './api';

function AdminGuard({ children }: { children: React.ReactNode }) {
  if (!getAdminPassword()) {
    return <Navigate to="/admin/login" replace />;
  }
  return <>{children}</>;
}

export function Router() {
  return (
    <Routes>
      <Route path="/explore" element={<Explore />} />
      <Route path="/advertiser" element={<Dashboard />} />
      <Route path="/publisher" element={<PublisherDashboard />} />
      <Route path="/publisher/login" element={<PublisherLogin />} />
      <Route path="/admin/login" element={<AdminLogin />} />
      <Route path="/admin" element={<AdminGuard><Overview /></AdminGuard>} />
      <Route path="/admin/auctions" element={<AdminGuard><AuctionLog /></AdminGuard>} />
      <Route path="/admin/advertisers" element={<AdminGuard><Advertisers /></AdminGuard>} />
      <Route path="/admin/publishers" element={<AdminGuard><Publishers /></AdminGuard>} />
      <Route path="/" element={<Navigate to="/admin" replace />} />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
