import { describe, it, expect, vi, beforeEach } from "vitest";
import { TEEClient, importRSAPublicKey, toBase64 } from "../src/tee.js";

// Generate a test RSA key pair for encryption tests
async function generateTestKeyPair() {
  const keyPair = await crypto.subtle.generateKey(
    { name: "RSA-OAEP", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
    true,
    ["encrypt", "decrypt"],
  );
  const pubKeyDer = await crypto.subtle.exportKey("spki", keyPair.publicKey);
  const pubKeyPem = `-----BEGIN PUBLIC KEY-----\n${toBase64(pubKeyDer)}\n-----END PUBLIC KEY-----`;
  return { keyPair, pubKeyPem };
}

describe("toBase64", () => {
  it("encodes an ArrayBuffer", () => {
    const buf = new TextEncoder().encode("hello");
    expect(toBase64(buf.buffer)).toBe(btoa("hello"));
  });
});

describe("importRSAPublicKey", () => {
  it("imports a PEM-encoded RSA public key", async () => {
    const { pubKeyPem } = await generateTestKeyPair();
    const key = await importRSAPublicKey(pubKeyPem);
    expect(key.type).toBe("public");
    expect(key.algorithm.name).toBe("RSA-OAEP");
  });
});

describe("TEEClient", () => {
  let client: TEEClient;
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    client = new TEEClient("http://localhost:8080");
    fetchSpy = vi.spyOn(globalThis, "fetch");
    vi.restoreAllMocks();
    fetchSpy = vi.spyOn(globalThis, "fetch");
  });

  it("starts without attestation", () => {
    expect(client.hasAttestation).toBe(false);
  });

  it("fetches attestation from server", async () => {
    const { pubKeyPem } = await generateTestKeyPair();

    fetchSpy.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          public_key: pubKeyPem,
          attestation_cose_base64: "mock-attestation",
        }),
        { status: 200 },
      ),
    );

    await client.fetchAttestation();
    expect(client.hasAttestation).toBe(true);
    expect(fetchSpy).toHaveBeenCalledWith("http://localhost:8080/tee/attestation");
  });

  it("throws on attestation error", async () => {
    fetchSpy.mockResolvedValueOnce(
      new Response("enclave unavailable", { status: 503 }),
    );

    await expect(client.fetchAttestation()).rejects.toThrow("enclave unavailable");
  });

  it("throws if encryptEmbedding called without attestation", async () => {
    await expect(client.encryptEmbedding([1, 2, 3])).rejects.toThrow(
      "No attestation",
    );
  });

  it("encrypts an embedding with hybrid RSA-OAEP + AES-GCM", async () => {
    const { keyPair, pubKeyPem } = await generateTestKeyPair();

    fetchSpy.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          public_key: pubKeyPem,
          attestation_cose_base64: "mock",
        }),
        { status: 200 },
      ),
    );

    await client.fetchAttestation();
    const encrypted = await client.encryptEmbedding([0.1, 0.2, 0.3]);

    // Verify structure
    expect(encrypted.aes_key_encrypted).toBeTruthy();
    expect(encrypted.encrypted_payload).toBeTruthy();
    expect(encrypted.nonce).toBeTruthy();
    expect(encrypted.hash_algorithm).toBe("SHA-256");

    // Verify we can decrypt the AES key with the private key
    const aesKeyRaw = await crypto.subtle.decrypt(
      { name: "RSA-OAEP" },
      keyPair.privateKey,
      Uint8Array.from(atob(encrypted.aes_key_encrypted), (c) => c.charCodeAt(0)),
    );

    // Verify we can decrypt the payload with the AES key
    const aesKey = await crypto.subtle.importKey(
      "raw",
      aesKeyRaw,
      { name: "AES-GCM" },
      false,
      ["decrypt"],
    );

    const nonce = Uint8Array.from(atob(encrypted.nonce), (c) => c.charCodeAt(0));
    const ciphertext = Uint8Array.from(atob(encrypted.encrypted_payload), (c) => c.charCodeAt(0));

    const decrypted = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv: nonce },
      aesKey,
      ciphertext,
    );

    const embedding = JSON.parse(new TextDecoder().decode(decrypted));
    expect(embedding).toEqual([0.1, 0.2, 0.3]);
  });
});
