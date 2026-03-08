import type { ChatMessage } from "@vectorspace/sdk";

/**
 * Send messages to /chat for a bot reply (no system prompt).
 * Returns the assistant's response content.
 * On 503 (no API key), returns a demo-mode fallback.
 */
export async function chatReply(messages: ChatMessage[]): Promise<string> {
  const resp = await fetch("/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ messages }),
  });

  if (!resp.ok) {
    if (resp.status === 503) {
      return "I'm running in demo mode — the chat API isn't available right now. Try loading a prebuilt conversation instead!";
    }
    const text = await resp.text().catch(() => "");
    throw new Error(text || `Chat error: ${resp.status}`);
  }

  const data = await resp.json();
  return data.content;
}
