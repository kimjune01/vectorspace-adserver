import { useState, useEffect, useCallback } from "react";
import type { Advertiser } from "./types";
import { API_BASE } from "./data";

export function useAdvertisers() {
  const [advertisers, setAdvertisers] = useState<Advertiser[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchAll = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}/positions`);
      if (!resp.ok) throw new Error(`Failed to fetch positions`);
      const data: Advertiser[] = await resp.json();
      setAdvertisers(data);
    } catch {
      // Server may not be running
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  const updateAdvertiser = useCallback(
    async (
      id: string,
      updates: Partial<{
        name: string;
        intent: string;
        sigma: number;
        bid_price: number;
        budget: number;
      }>
    ) => {
      const resp = await fetch(`${API_BASE}/advertiser/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updates),
      });
      if (!resp.ok) throw new Error(`Update failed`);
      await fetchAll();
    },
    [fetchAll]
  );

  const deleteAdvertiser = useCallback(
    async (id: string) => {
      const resp = await fetch(`${API_BASE}/advertiser/${id}`, {
        method: "DELETE",
      });
      if (!resp.ok) throw new Error(`Delete failed`);
      await fetchAll();
    },
    [fetchAll]
  );

  const addAdvertiser = useCallback(
    async (adv: {
      name: string;
      intent: string;
      sigma: number;
      bid_price: number;
      budget: number;
    }) => {
      const resp = await fetch(`${API_BASE}/advertiser/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...adv, currency: "USD" }),
      });
      if (!resp.ok) throw new Error(`Register failed`);
      await fetchAll();
    },
    [fetchAll]
  );

  return { advertisers, loading, updateAdvertiser, deleteAdvertiser, addAdvertiser, refresh: fetchAll };
}
