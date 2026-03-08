# Reference SDK: sdk-web/ (TypeScript). This SDK follows the API surface defined there.
"""CloudX Publisher SDK — Python client."""

from __future__ import annotations

import urllib.request
import urllib.error
import json


class CloudX:
    """Minimal Python client for the CloudX ad server."""

    def __init__(self, endpoint: str) -> None:
        self.endpoint = endpoint.rstrip("/")
        self._embedding_cache: list[dict] = []
        self._embedding_etag: str | None = None

    # ── Embedding cache ──────────────────────────────────────────────

    def sync_embeddings(self) -> None:
        """Fetch advertiser embeddings. Uses ETag for 304 caching."""
        headers: dict[str, str] = {}
        if self._embedding_etag:
            headers["If-None-Match"] = self._embedding_etag

        req = urllib.request.Request(
            f"{self.endpoint}/embeddings", headers=headers
        )
        try:
            resp = urllib.request.urlopen(req)
        except urllib.error.HTTPError as e:
            if e.code == 304:
                return  # cache is fresh
            raise

        data = json.loads(resp.read())
        self._embedding_cache = data["embeddings"]
        etag = resp.headers.get("ETag")
        if etag:
            self._embedding_etag = etag

    def embed(self, text: str) -> list[float]:
        """Embed arbitrary text via the server's embedding sidecar."""
        body = json.dumps({"text": text}).encode()
        req = urllib.request.Request(
            f"{self.endpoint}/embed",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        resp = urllib.request.urlopen(req)
        data = json.loads(resp.read())
        return data["embedding"]

    def proximity(self, query_embedding: list[float]) -> list[dict]:
        """Squared Euclidean distance to each cached embedding, sorted ascending."""
        results = []
        for entry in self._embedding_cache:
            dist = _squared_euclidean(query_embedding, entry["embedding"])
            results.append({"id": entry["id"], "distance": dist})
        results.sort(key=lambda r: r["distance"])
        return results

    # ── Ad requests ──────────────────────────────────────────────────

    def request_ad(
        self, intent: str, tau: float | None = None, publisher_id: str | None = None
    ) -> dict | None:
        """Request an ad for the given intent."""
        payload: dict = {"intent": intent}
        if tau is not None and tau > 0:
            payload["tau"] = tau
        if publisher_id is not None:
            payload["publisher_id"] = publisher_id

        body = json.dumps(payload).encode()
        req = urllib.request.Request(
            f"{self.endpoint}/ad-request",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            resp = urllib.request.urlopen(req)
        except urllib.error.HTTPError as e:
            if e.code == 500:
                err_body = e.read().decode()
                if "no bidders passed" in err_body:
                    return None
            raise
        return json.loads(resp.read())


    # ── Event Tracking ─────────────────────────────────────────────

    def report_impression(
        self,
        auction_id: int,
        advertiser_id: str,
        user_id: str | None = None,
        publisher_id: str | None = None,
    ) -> bool:
        """Report an impression. Returns False if frequency-capped (429)."""
        payload: dict = {"auction_id": auction_id, "advertiser_id": advertiser_id}
        if user_id is not None:
            payload["user_id"] = user_id
        if publisher_id is not None:
            payload["publisher_id"] = publisher_id

        body = json.dumps(payload).encode()
        req = urllib.request.Request(
            f"{self.endpoint}/event/impression",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            urllib.request.urlopen(req)
            return True
        except urllib.error.HTTPError as e:
            if e.code == 429:
                return False
            raise

    def report_click(
        self,
        auction_id: int,
        advertiser_id: str,
        user_id: str | None = None,
        publisher_id: str | None = None,
    ) -> None:
        """Report a click event."""
        payload: dict = {"auction_id": auction_id, "advertiser_id": advertiser_id}
        if user_id is not None:
            payload["user_id"] = user_id
        if publisher_id is not None:
            payload["publisher_id"] = publisher_id

        body = json.dumps(payload).encode()
        req = urllib.request.Request(
            f"{self.endpoint}/event/click",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        urllib.request.urlopen(req)

    def report_viewable(
        self,
        auction_id: int,
        advertiser_id: str,
        user_id: str | None = None,
        publisher_id: str | None = None,
    ) -> None:
        """Report a viewability event."""
        payload: dict = {"auction_id": auction_id, "advertiser_id": advertiser_id}
        if user_id is not None:
            payload["user_id"] = user_id
        if publisher_id is not None:
            payload["publisher_id"] = publisher_id

        body = json.dumps(payload).encode()
        req = urllib.request.Request(
            f"{self.endpoint}/event/viewable",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        urllib.request.urlopen(req)


def _squared_euclidean(a: list[float], b: list[float]) -> float:
    """||a - b||²"""
    return sum((ai - bi) ** 2 for ai, bi in zip(a, b))
