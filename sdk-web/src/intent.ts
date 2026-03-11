import type { ChatMessage } from "./types.js";

const INTENT_PROMPT = `Given a conversation, decide whether the person could benefit from a professional service. If yes, write a single sentence describing that service — as if the provider were writing their own position statement. If the conversation is casual, off-topic, or doesn't suggest any professional need, respond with exactly "NONE".

Format: [value prop] + [ideal client profile] + [qualifier]
Example: "Sports injury knee rehab for competitive endurance athletes recovering from overuse."

Rules:
- Match the most obvious need. A health complaint needs a health provider, not a lawyer. A legal issue needs legal help, not a therapist.
- Be specific to the situation but don't embellish beyond what's stated.
- Do NOT extract demographics or personal data about the user.
- If there is no clear professional need, respond with "NONE".

Respond with ONLY the one-sentence service description or "NONE", nothing else.`;

/**
 * Extract a service-description intent from a chat conversation.
 * Calls the server's /chat endpoint with the INTENT_PROMPT system message.
 * Returns the intent string or "NONE" if no professional need is detected.
 */
export async function extractIntent(
  endpoint: string,
  messages: ChatMessage[],
): Promise<string> {
  const resp = await fetch(`${endpoint}/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ messages, system: INTENT_PROMPT }),
  });

  if (!resp.ok) {
    const text = await resp.text().catch(() => "");
    throw new Error(text || `Chat API error: ${resp.status}`);
  }

  const data = await resp.json();
  return data.content;
}
