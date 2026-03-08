import type {
  AuctionRow,
  RevenuePeriod,
  AdvertiserSpend,
  AdvertiserWithBudget,
  Creative,
  EventStats,
  PortalProfile,
  PublisherInfo,
  PublisherProfile,
  PublisherStats,
} from './types';

const API = import.meta.env.VITE_API_URL || 'http://localhost:8080';

// --- Portal (token-authenticated) ---

export async function getProfile(token: string): Promise<PortalProfile> {
  const resp = await fetch(`${API}/portal/me?token=${token}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function updateProfile(
  token: string,
  data: Partial<{ name: string; intent: string; sigma: number; bid_price: number; budget: number; url: string }>
): Promise<unknown> {
  const resp = await fetch(`${API}/portal/me?token=${token}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getMyAuctions(
  token: string,
  limit = 20,
  offset = 0
): Promise<{ auctions: AuctionRow[] | null; total: number }> {
  const resp = await fetch(`${API}/portal/me/auctions?token=${token}&limit=${limit}&offset=${offset}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function downloadMyAuctionsCSV(token: string): Promise<void> {
  const resp = await fetch(`${API}/portal/me/auctions?token=${token}&format=csv`);
  if (!resp.ok) throw new Error(await resp.text());
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'my-auctions.csv';
  a.click();
  URL.revokeObjectURL(url);
}

export async function getMyEvents(token: string): Promise<EventStats> {
  const resp = await fetch(`${API}/portal/me/events?token=${token}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

// --- Creatives ---

export async function getCreatives(token: string): Promise<Creative[]> {
  const resp = await fetch(`${API}/portal/me/creatives?token=${token}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function createCreative(
  token: string,
  title: string,
  subtitle: string
): Promise<Creative> {
  const resp = await fetch(`${API}/portal/me/creatives?token=${token}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, subtitle }),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function updateCreative(
  token: string,
  id: number,
  title: string,
  subtitle: string
): Promise<Creative> {
  const resp = await fetch(`${API}/portal/me/creatives/${id}?token=${token}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, subtitle }),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function deleteCreative(token: string, id: number): Promise<void> {
  const resp = await fetch(`${API}/portal/me/creatives/${id}?token=${token}`, {
    method: 'DELETE',
  });
  if (!resp.ok) throw new Error(await resp.text());
}

// --- Admin Auth ---

export function getAdminPassword(): string | null {
  return localStorage.getItem('admin_password');
}

function adminHeaders(): Record<string, string> {
  const pw = getAdminPassword();
  return pw ? { 'X-Admin-Password': pw } : {};
}

async function adminFetch(url: string, init?: RequestInit): Promise<Response> {
  const resp = await fetch(url, { ...init, headers: { ...adminHeaders(), ...init?.headers } });
  if (resp.status === 401) {
    adminLogout();
    window.location.href = '/admin/login';
    throw new Error('Unauthorized');
  }
  return resp;
}

export async function adminLogin(password: string): Promise<void> {
  const resp = await fetch(`${API}/admin/auctions?limit=1`, {
    headers: { 'X-Admin-Password': password },
  });
  if (!resp.ok) throw new Error('Invalid password');
  localStorage.setItem('admin_password', password);
}

export function adminLogout(): void {
  localStorage.removeItem('admin_password');
}

// --- Admin ---

export async function getAdminAuctions(
  limit = 20,
  offset = 0,
  winner = '',
  intent = ''
): Promise<{ auctions: AuctionRow[] | null; total: number }> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (winner) params.set('winner', winner);
  if (intent) params.set('intent', intent);
  const resp = await adminFetch(`${API}/admin/auctions?${params}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function downloadAdminAuctionsCSV(winner = '', intent = ''): Promise<void> {
  const params = new URLSearchParams({ format: 'csv' });
  if (winner) params.set('winner', winner);
  if (intent) params.set('intent', intent);
  const resp = await adminFetch(`${API}/admin/auctions?${params}`);
  if (!resp.ok) throw new Error(await resp.text());
  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'auctions.csv';
  a.click();
  URL.revokeObjectURL(url);
}

export async function getAdminRevenue(
  groupBy: 'day' | 'week' | 'month' = 'day'
): Promise<{ group_by: string; periods: RevenuePeriod[] | null }> {
  const resp = await adminFetch(`${API}/admin/revenue?group_by=${groupBy}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getAdminTopAdvertisers(limit = 10): Promise<AdvertiserSpend[]> {
  const resp = await adminFetch(`${API}/admin/top-advertisers?limit=${limit}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getAdminAdvertisers(): Promise<AdvertiserWithBudget[]> {
  const resp = await adminFetch(`${API}/admin/advertisers`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getAdminEvents(): Promise<EventStats> {
  const resp = await adminFetch(`${API}/admin/events`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getAdminPublishers(): Promise<PublisherInfo[]> {
  const resp = await adminFetch(`${API}/admin/publishers`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

// --- Publisher Auth ---

export async function publisherLogin(
  email: string,
  password: string
): Promise<{ token: string; publisher_id: string }> {
  const resp = await fetch(`${API}/publisher/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

// --- Publisher Portal (token-authenticated) ---

export async function getPublisherProfile(token: string): Promise<PublisherProfile> {
  const resp = await fetch(`${API}/portal/publisher/me?token=${token}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getPublisherStats(token: string): Promise<PublisherStats> {
  const resp = await fetch(`${API}/portal/publisher/stats?token=${token}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getPublisherRevenue(
  token: string,
  groupBy: 'day' | 'week' | 'month' = 'day'
): Promise<{ group_by: string; periods: RevenuePeriod[] | null }> {
  const resp = await fetch(`${API}/portal/publisher/revenue?token=${token}&group_by=${groupBy}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getPublisherEvents(token: string): Promise<EventStats> {
  const resp = await fetch(`${API}/portal/publisher/events?token=${token}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getPublisherAuctions(
  token: string,
  limit = 20,
  offset = 0
): Promise<{ auctions: AuctionRow[] | null; total: number }> {
  const resp = await fetch(`${API}/portal/publisher/auctions?token=${token}&limit=${limit}&offset=${offset}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

export async function getPublisherTopAdvertisers(
  token: string,
  limit = 10
): Promise<AdvertiserSpend[]> {
  const resp = await fetch(`${API}/portal/publisher/top-advertisers?token=${token}&limit=${limit}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

// --- Stats ---

export async function getStats(): Promise<{
  auction_count: number;
  total_spend: number;
  publisher_revenue: number;
  exchange_revenue: number;
}> {
  const resp = await fetch(`${API}/stats`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}
