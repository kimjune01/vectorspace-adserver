import type { EncryptedEmbedding } from "./types.js";

/** Import a PEM-encoded RSA public key into Web Crypto for RSA-OAEP(SHA-256). */
export async function importRSAPublicKey(pem: string): Promise<CryptoKey> {
  const pemBody = pem
    .replace(/-----BEGIN PUBLIC KEY-----/, "")
    .replace(/-----END PUBLIC KEY-----/, "")
    .replace(/\s/g, "");
  const binaryDer = Uint8Array.from(atob(pemBody), (c) => c.charCodeAt(0));

  return crypto.subtle.importKey(
    "spki",
    binaryDer.buffer,
    { name: "RSA-OAEP", hash: "SHA-256" },
    false,
    ["encrypt"],
  );
}

/** Convert an ArrayBuffer to base64 string. */
export function toBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

/** TEE attestation and encryption client. */
export class TEEClient {
  private pubKey: CryptoKey | null = null;
  private attestation: {
    public_key: string;
    attestation_cose_base64: string;
  } | null = null;

  constructor(private endpoint: string) {}

  /** Whether an attestation has been fetched. */
  get hasAttestation(): boolean {
    return this.pubKey !== null;
  }

  /** Fetch the enclave's attested public key. */
  async fetchAttestation(): Promise<void> {
    const resp = await fetch(`${this.endpoint}/tee/attestation`);
    if (!resp.ok) {
      const text = await resp.text().catch(() => "");
      throw new Error(text || `Attestation error: ${resp.status}`);
    }

    const data: { public_key: string; attestation_cose_base64: string } =
      await resp.json();
    this.attestation = data;
    this.pubKey = await importRSAPublicKey(data.public_key);
  }

  /**
   * Encrypt an embedding vector using the enclave's attested public key.
   * Hybrid RSA-OAEP(SHA-256) + AES-256-GCM via Web Crypto API.
   */
  async encryptEmbedding(embedding: number[]): Promise<EncryptedEmbedding> {
    if (!this.pubKey) {
      throw new Error("No attestation — call fetchAttestation() first");
    }

    // Generate random AES-256 key
    const aesKey = await crypto.subtle.generateKey(
      { name: "AES-GCM", length: 256 },
      true,
      ["encrypt"],
    );

    // Export AES key as raw bytes
    const aesKeyRaw = await crypto.subtle.exportKey("raw", aesKey);

    // Encrypt AES key with RSA-OAEP(SHA-256)
    const aesKeyEncrypted = await crypto.subtle.encrypt(
      { name: "RSA-OAEP" },
      this.pubKey,
      aesKeyRaw,
    );

    // Encrypt embedding payload with AES-256-GCM
    const nonce = crypto.getRandomValues(new Uint8Array(12));
    const payload = new TextEncoder().encode(JSON.stringify(embedding));
    const encryptedPayload = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv: nonce },
      aesKey,
      payload,
    );

    return {
      aes_key_encrypted: toBase64(aesKeyEncrypted),
      encrypted_payload: toBase64(encryptedPayload),
      nonce: toBase64(nonce.buffer),
      hash_algorithm: "SHA-256",
    };
  }
}
