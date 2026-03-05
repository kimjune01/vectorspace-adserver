"""Unit tests for the CloudX SDK client using a mock HTTP transport."""

import json

import httpx
import pytest

from cloudx import CloudXClient


class MockTransport(httpx.BaseTransport):
    """Records requests and returns canned responses."""

    def __init__(self) -> None:
        self.requests: list[tuple[str, str, dict]] = []
        self.responses: dict[str, httpx.Response] = {}

    def set_response(self, path: str, *, status: int = 200, body: dict | list) -> None:
        self.responses[path] = httpx.Response(
            status_code=status,
            json=body,
        )

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content) if request.content else {}
        self.requests.append((request.method, request.url.raw_path.decode(), body))
        path = request.url.raw_path.decode()
        if path in self.responses:
            return self.responses[path]
        return httpx.Response(status_code=404, json={"error": "not found"})


@pytest.fixture
def mock_client():
    transport = MockTransport()
    transport.set_response(
        "/ad-request",
        status=200,
        body={
            "winner_id": "adv-1",
            "winner_name": "Peak PT",
            "payment": 1.42,
            "currency": "USD",
            "bid_count": 3,
            "eligible_count": 2,
        },
    )
    transport.set_response(
        "/advertiser/register",
        status=201,
        body={
            "id": "adv-1",
            "name": "Peak PT",
            "intent": "sports injury knee rehab for competitive athletes",
            "sigma": 0.5,
            "bid_price": 2.5,
            "currency": "USD",
        },
    )
    transport.set_response(
        "/positions",
        body=[
            {"id": "adv-1", "name": "Peak PT", "intent": "knee rehab", "sigma": 0.5, "bid_price": 2.5},
        ],
    )
    transport.set_response(
        "/budget/adv-1",
        body={"advertiser_id": "adv-1", "total": 100, "spent": 1.42, "remaining": 98.58, "currency": "USD"},
    )

    client = CloudXClient("http://localhost:8080")
    client._client = httpx.Client(transport=transport, base_url="http://localhost:8080")
    yield client, transport
    client.close()


def test_get_ad_sends_intent(mock_client):
    client, transport = mock_client
    result = client.get_ad("knee rehab for marathon runners")

    assert result["winner_id"] == "adv-1"
    assert result["winner_name"] == "Peak PT"
    method, path, body = transport.requests[-1]
    assert method == "POST"
    assert path == "/ad-request"
    assert body == {"intent": "knee rehab for marathon runners"}


def test_register_sends_intent_not_embedding(mock_client):
    client, transport = mock_client
    result = client.register(
        name="Peak PT",
        intent="sports injury knee rehab for competitive athletes",
        sigma=0.5,
        bid_price=2.5,
        budget=100,
    )

    assert result["id"] == "adv-1"
    assert result["intent"] == "sports injury knee rehab for competitive athletes"
    assert "embedding" not in result

    _, _, body = transport.requests[-1]
    assert body["intent"] == "sports injury knee rehab for competitive athletes"
    assert "embedding" not in body


def test_positions_returns_intents(mock_client):
    client, _ = mock_client
    positions = client.positions()
    assert len(positions) == 1
    assert positions[0]["intent"] == "knee rehab"
    assert "embedding" not in positions[0]


def test_budget_returns_info(mock_client):
    client, _ = mock_client
    info = client.budget("adv-1")
    assert info["remaining"] == 98.58


def test_context_manager():
    transport = MockTransport()
    with CloudXClient("http://localhost:8080") as client:
        client._client = httpx.Client(transport=transport, base_url="http://localhost:8080")
