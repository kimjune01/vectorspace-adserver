"""Thin HTTP client for the CloudX ad server."""

from __future__ import annotations

import httpx


class CloudXClient:
    """CloudX ad server client for publishers and advertisers."""

    def __init__(self, base_url: str, timeout: float = 10.0) -> None:
        self._base = base_url.rstrip("/")
        self._client = httpx.Client(base_url=self._base, timeout=timeout)

    # ── Publisher ──────────────────────────────────────────────

    def get_ad(self, intent: str) -> dict:
        """Request an ad for the given intent string."""
        resp = self._client.post("/ad-request", json={"intent": intent})
        resp.raise_for_status()
        return resp.json()

    # ── Advertiser ────────────────────────────────────────────

    def register(
        self,
        name: str,
        intent: str,
        sigma: float = 0.5,
        bid_price: float = 1.0,
        budget: float = 100.0,
        currency: str = "USD",
    ) -> dict:
        """Register a new advertiser position."""
        resp = self._client.post(
            "/advertiser/register",
            json={
                "name": name,
                "intent": intent,
                "sigma": sigma,
                "bid_price": bid_price,
                "budget": budget,
                "currency": currency,
            },
        )
        resp.raise_for_status()
        return resp.json()

    # ── Read-only ─────────────────────────────────────────────

    def positions(self) -> list[dict]:
        """List all registered advertiser positions."""
        resp = self._client.get("/positions")
        resp.raise_for_status()
        return resp.json()

    def budget(self, advertiser_id: str) -> dict:
        """Get budget info for an advertiser."""
        resp = self._client.get(f"/budget/{advertiser_id}")
        resp.raise_for_status()
        return resp.json()

    def close(self) -> None:
        self._client.close()

    def __enter__(self) -> CloudXClient:
        return self

    def __exit__(self, *args: object) -> None:
        self.close()
