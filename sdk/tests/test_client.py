"""Tests for VectorSpace Python SDK."""

import json
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from unittest import TestCase

import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from vectorspace.client import VectorSpace


class FakeHandler(BaseHTTPRequestHandler):
    """Minimal mock server for SDK tests."""

    etag = '"test-version-hash"'

    def do_GET(self):
        if self.path == "/embeddings":
            inm = self.headers.get("If-None-Match")
            if inm == self.etag:
                self.send_response(304)
                self.end_headers()
                return

            body = json.dumps({
                "version": "test-version-hash",
                "embeddings": [
                    {"id": "adv-1", "embedding": [0.1, 0.2, 0.3]},
                    {"id": "adv-2", "embedding": [0.9, 0.8, 0.7]},
                ],
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("ETag", self.etag)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)

        if self.path == "/embed":
            req = json.loads(raw) if raw else {}
            if not req.get("text"):
                self.send_error(400, "text is required")
                return
            body = json.dumps({"embedding": [0.5, 0.5, 0.5]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_error(404)

    def log_message(self, format, *args):
        pass  # suppress logs


class TestVectorSpace(TestCase):
    server: HTTPServer
    thread: threading.Thread
    endpoint: str

    @classmethod
    def setUpClass(cls):
        cls.server = HTTPServer(("127.0.0.1", 0), FakeHandler)
        port = cls.server.server_address[1]
        cls.endpoint = f"http://127.0.0.1:{port}"
        cls.thread = threading.Thread(target=cls.server.serve_forever)
        cls.thread.daemon = True
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def new_client(self) -> VectorSpace:
        return VectorSpace(self.endpoint)

    # ── sync_embeddings ──────────────────────────────────────────

    def test_sync_embeddings_fetches(self):
        c = self.new_client()
        c.sync_embeddings()
        results = c.proximity([0.1, 0.2, 0.3])
        self.assertEqual(len(results), 2)
        self.assertEqual(results[0]["id"], "adv-1")

    def test_sync_embeddings_304_on_second_call(self):
        c = self.new_client()
        c.sync_embeddings()  # first → 200
        c.sync_embeddings()  # second → 304 (no error)
        # cache should still be intact
        results = c.proximity([0.1, 0.2, 0.3])
        self.assertEqual(len(results), 2)

    # ── embed ────────────────────────────────────────────────────

    def test_embed_returns_vector(self):
        c = self.new_client()
        vec = c.embed("back pain from sitting")
        self.assertEqual(vec, [0.5, 0.5, 0.5])

    # ── proximity ────────────────────────────────────────────────

    def test_proximity_empty_cache(self):
        c = self.new_client()
        self.assertEqual(c.proximity([0.5, 0.5, 0.5]), [])

    def test_proximity_sorts_ascending(self):
        c = self.new_client()
        c.sync_embeddings()
        # adv-1 at [0.1, 0.2, 0.3], adv-2 at [0.9, 0.8, 0.7]
        # query [0.1, 0.2, 0.3] → dist to adv-1 = 0, dist to adv-2 ≈ 1.16
        results = c.proximity([0.1, 0.2, 0.3])
        self.assertEqual(results[0]["id"], "adv-1")
        self.assertAlmostEqual(results[0]["distance"], 0.0, places=5)
        self.assertEqual(results[1]["id"], "adv-2")
        self.assertAlmostEqual(results[1]["distance"], 1.16, places=2)

    def test_proximity_closest_changes_with_query(self):
        c = self.new_client()
        c.sync_embeddings()
        # query near adv-2
        results = c.proximity([0.9, 0.8, 0.7])
        self.assertEqual(results[0]["id"], "adv-2")
        self.assertAlmostEqual(results[0]["distance"], 0.0, places=5)
