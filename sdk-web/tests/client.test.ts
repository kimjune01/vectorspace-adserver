import { describe, it, expect, vi, beforeEach } from "vitest";
import { VectorSpace } from "../src/client.js";

describe("VectorSpace", () => {
  let vs: VectorSpace;
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vs = new VectorSpace({
      endpoint: "http://localhost:8080",
      publisherId: "pub-1",
    });
    vi.restoreAllMocks();
    fetchSpy = vi.spyOn(globalThis, "fetch");
  });

  describe("constructor", () => {
    it("strips trailing slashes from endpoint", () => {
      const vs2 = new VectorSpace({ endpoint: "http://example.com///" });
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ embeddings: [] }), {
          status: 200,
          headers: { ETag: '"v1"' },
        }),
      );
      vs2.syncEmbeddings();
      expect(fetchSpy.mock.calls[0][0]).toBe("http://example.com/embeddings");
    });
  });

  describe("embed", () => {
    it("calls POST /embed and returns vector", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ embedding: [0.1, 0.2, 0.3] }), {
          status: 200,
        }),
      );

      const result = await vs.embed("hello world");
      expect(result).toEqual([0.1, 0.2, 0.3]);

      const [url, opts] = fetchSpy.mock.calls[0];
      expect(url).toBe("http://localhost:8080/embed");
      expect(JSON.parse(opts?.body as string)).toEqual({ text: "hello world" });
    });

    it("throws on error", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("sidecar down", { status: 500 }),
      );

      await expect(vs.embed("test")).rejects.toThrow("sidecar down");
    });
  });

  describe("requestAd", () => {
    it("sends intent and publisherId to /ad-request", async () => {
      const adResponse = {
        auction_id: 1,
        intent: "dog training",
        winner: { id: "adv-1", rank: 1, name: "Dog Trainer", intent: "dog training", bid_price: 5, sigma: 1, score: 1, distance_sq: 0.1, log_bid: 1 },
        runner_up: null,
        all_bidders: [],
        payment: 5,
        currency: "USD",
        bid_count: 1,
        eligible_count: 1,
      };

      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify(adResponse), { status: 200 }),
      );

      const result = await vs.requestAd({ intent: "dog training", tau: 0.5 });
      expect(result).toEqual(adResponse);

      const body = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(body.intent).toBe("dog training");
      expect(body.tau).toBe(0.5);
      expect(body.publisher_id).toBe("pub-1");
    });

    it("returns null when no bidders pass", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("no bidders passed", { status: 500 }),
      );

      const result = await vs.requestAd({ intent: "obscure query" });
      expect(result).toBeNull();
    });

    it("omits tau when not provided", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ auction_id: 1 }), { status: 200 }),
      );

      await vs.requestAd({ intent: "test" });
      const body = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(body).not.toHaveProperty("tau");
    });

    it("throws on non-500 error", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("bad request", { status: 400 }),
      );

      await expect(vs.requestAd({ intent: "test" })).rejects.toThrow("bad request");
    });
  });

  describe("extractIntent", () => {
    it("calls /chat with messages and returns intent", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(
          JSON.stringify({ content: "Physical therapist specializing in knee recovery" }),
          { status: 200 },
        ),
      );

      const intent = await vs.extractIntent([
        { role: "user", content: "My knee hurts after running" },
      ]);

      expect(intent).toBe("Physical therapist specializing in knee recovery");
      const body = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(body.messages).toHaveLength(1);
      expect(body.system).toBeTruthy();
    });

    it("throws on /chat error", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response("chat endpoint down", { status: 503 }),
      );

      await expect(
        vs.extractIntent([{ role: "user", content: "hello" }]),
      ).rejects.toThrow("chat endpoint down");
    });
  });

  describe("requestAdFromChat", () => {
    it("returns null when intent is NONE", async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ content: "NONE" }), { status: 200 }),
      );

      const result = await vs.requestAdFromChat([
        { role: "user", content: "Tell me a joke" },
      ]);

      expect(result).toBeNull();
      expect(fetchSpy).toHaveBeenCalledTimes(1); // only /chat, no /ad-request
    });

    it("chains extractIntent and requestAd", async () => {
      fetchSpy
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ content: "Dog trainer" }), { status: 200 }),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ auction_id: 1, winner: { id: "adv-1" } }), { status: 200 }),
        );

      const result = await vs.requestAdFromChat(
        [{ role: "user", content: "I need help training my dog" }],
        0.5,
      );

      expect(result).toBeTruthy();
      expect(fetchSpy).toHaveBeenCalledTimes(2);
      // Second call should be to /ad-request with the extracted intent
      const adBody = JSON.parse(fetchSpy.mock.calls[1][1]?.body as string);
      expect(adBody.intent).toBe("Dog trainer");
      expect(adBody.tau).toBe(0.5);
    });
  });

  describe("fetchAttestation", () => {
    it("delegates to TEEClient", async () => {
      // Generate a real RSA key pair for the mock response
      const keyPair = await crypto.subtle.generateKey(
        { name: "RSA-OAEP", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
        true,
        ["encrypt", "decrypt"],
      );
      const pubKeyDer = await crypto.subtle.exportKey("spki", keyPair.publicKey);
      const bytes = new Uint8Array(pubKeyDer);
      let binary = "";
      for (let i = 0; i < bytes.byteLength; i++) binary += String.fromCharCode(bytes[i]);
      const pubKeyPem = `-----BEGIN PUBLIC KEY-----\n${btoa(binary)}\n-----END PUBLIC KEY-----`;

      fetchSpy.mockResolvedValueOnce(
        new Response(
          JSON.stringify({ public_key: pubKeyPem, attestation_cose_base64: "mock" }),
          { status: 200 },
        ),
      );

      await vs.fetchAttestation();
      expect(fetchSpy.mock.calls[0][0]).toBe("http://localhost:8080/tee/attestation");
    });
  });

  describe("requestAdTEE", () => {
    async function setupTEE() {
      const keyPair = await crypto.subtle.generateKey(
        { name: "RSA-OAEP", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
        true,
        ["encrypt", "decrypt"],
      );
      const pubKeyDer = await crypto.subtle.exportKey("spki", keyPair.publicKey);
      const bytes = new Uint8Array(pubKeyDer);
      let binary = "";
      for (let i = 0; i < bytes.byteLength; i++) binary += String.fromCharCode(bytes[i]);
      const pubKeyPem = `-----BEGIN PUBLIC KEY-----\n${btoa(binary)}\n-----END PUBLIC KEY-----`;
      return { keyPair, pubKeyPem };
    }

    it("auto-fetches attestation if not present, embeds, encrypts, and sends ad request", async () => {
      const { pubKeyPem } = await setupTEE();

      fetchSpy
        // 1st call: /tee/attestation (auto-fetch)
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({ public_key: pubKeyPem, attestation_cose_base64: "mock" }),
            { status: 200 },
          ),
        )
        // 2nd call: /embed
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ embedding: [0.1, 0.2, 0.3] }), { status: 200 }),
        )
        // 3rd call: /ad-request with encrypted payload
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({ auction_id: 99, winner: { id: "adv-1" }, payment: 3.5 }),
            { status: 200 },
          ),
        );

      const result = await vs.requestAdTEE({ intent: "dog training", tau: 0.5 });

      expect(result).toBeTruthy();
      expect(result!.auction_id).toBe(99);

      // Verify 3 fetch calls: attestation, embed, ad-request
      expect(fetchSpy).toHaveBeenCalledTimes(3);
      expect(fetchSpy.mock.calls[0][0]).toBe("http://localhost:8080/tee/attestation");
      expect(fetchSpy.mock.calls[1][0]).toBe("http://localhost:8080/embed");
      expect(fetchSpy.mock.calls[2][0]).toBe("http://localhost:8080/ad-request");

      // Verify the ad-request body has encrypted fields
      const adBody = JSON.parse(fetchSpy.mock.calls[2][1]?.body as string);
      expect(adBody.encrypted_embedding).toBeTruthy();
      expect(adBody.encrypted_embedding.aes_key_encrypted).toBeTruthy();
      expect(adBody.encrypted_embedding.encrypted_payload).toBeTruthy();
      expect(adBody.encrypted_embedding.nonce).toBeTruthy();
      expect(adBody.tau).toBe(0.5);
      expect(adBody.publisher_id).toBe("pub-1");
    });

    it("returns null when TEE auction has no bidders", async () => {
      const { pubKeyPem } = await setupTEE();

      fetchSpy
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({ public_key: pubKeyPem, attestation_cose_base64: "mock" }),
            { status: 200 },
          ),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ embedding: [0.1, 0.2] }), { status: 200 }),
        )
        .mockResolvedValueOnce(
          new Response("no bidders passed", { status: 500 }),
        );

      const result = await vs.requestAdTEE({ intent: "obscure" });
      expect(result).toBeNull();
    });

    it("throws on non-500 TEE error", async () => {
      const { pubKeyPem } = await setupTEE();

      fetchSpy
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({ public_key: pubKeyPem, attestation_cose_base64: "mock" }),
            { status: 200 },
          ),
        )
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ embedding: [0.1] }), { status: 200 }),
        )
        .mockResolvedValueOnce(
          new Response("server error", { status: 502 }),
        );

      await expect(vs.requestAdTEE({ intent: "test" })).rejects.toThrow("server error");
    });
  });

  describe("event tracking", () => {
    it("delegates reportImpression to EventTracker", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      const result = await vs.reportImpression(42, "adv-1", "user-1");
      expect(result).toBe(true);
      expect(fetchSpy.mock.calls[0][0]).toBe(
        "http://localhost:8080/event/impression",
      );
    });

    it("delegates reportClick to EventTracker", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await vs.reportClick(42, "adv-1");
      expect(fetchSpy.mock.calls[0][0]).toBe(
        "http://localhost:8080/event/click",
      );
    });

    it("delegates reportViewable to EventTracker", async () => {
      fetchSpy.mockResolvedValueOnce(new Response("ok", { status: 200 }));

      await vs.reportViewable(42, "adv-1");
      expect(fetchSpy.mock.calls[0][0]).toBe(
        "http://localhost:8080/event/viewable",
      );
    });
  });

  describe("syncEmbeddings + proximity", () => {
    it("syncs and computes proximity", async () => {
      const embeddings = [
        { id: "adv-1", name: "A", embedding: [1, 0], bid_price: 5, sigma: 1, currency: "USD" },
        { id: "adv-2", name: "B", embedding: [0, 1], bid_price: 3, sigma: 1, currency: "USD" },
      ];

      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ embeddings }), {
          status: 200,
          headers: { ETag: '"v1"' },
        }),
      );

      await vs.syncEmbeddings();

      const results = vs.proximity([1, 0]);
      expect(results[0].id).toBe("adv-1");
      expect(results[0].distance).toBe(0);
      expect(results[1].id).toBe("adv-2");
      expect(results[1].distance).toBe(2); // (1-0)^2 + (0-1)^2
    });
  });
});
