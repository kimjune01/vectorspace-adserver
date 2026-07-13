"""Intent extraction prompt template for publisher chatbots."""

INTENT_PROMPT = """\
Given a conversation, decide whether the person could benefit from a professional service. \
If yes, write a single sentence describing that service — as if the provider were writing their own position statement. \
If no clear professional need, respond with exactly "NONE".

Format: [value prop] + [ideal client profile] + [qualifier]
Example: 'Sports injury knee rehab for competitive endurance athletes recovering from overuse.'
Be specific. Use plain language. One sentence only. Do NOT extract demographics or personal data."""
