import { describe, it, expect } from "vitest";
import { conversations, type Conversation } from "../src/conversations";

describe("conversations", () => {
  it("has 8 prebuilt conversations", () => {
    expect(conversations).toHaveLength(8);
  });

  it("each conversation has a label and at least 4 messages", () => {
    for (const c of conversations) {
      expect(c.label).toBeTruthy();
      expect(c.messages.length).toBeGreaterThanOrEqual(4);
    }
  });

  it("each message alternates user/assistant starting with user", () => {
    for (const c of conversations) {
      for (let i = 0; i < c.messages.length; i++) {
        const expected = i % 2 === 0 ? "user" : "assistant";
        expect(c.messages[i].role).toBe(expected);
      }
    }
  });

  it("includes health topics", () => {
    const labels = conversations.map((c) => c.label);
    expect(labels).toContain("Anxiety & Sleep");
    expect(labels).toContain("High Cholesterol");
    expect(labels).toContain("Back Pain");
    expect(labels).toContain("Loneliness After Moving");
    expect(labels).toContain("Relationship Conflict");
    expect(labels).toContain("Quitting Smoking");
  });

  it("includes off-topic conversations", () => {
    const labels = conversations.map((c) => c.label);
    expect(labels).toContain("Just Chatting: Recipes");
    expect(labels).toContain("Just Chatting: Movies");
  });

  it("off-topic conversations are marked as offTopic", () => {
    const offTopic = conversations.filter((c) => c.offTopic);
    expect(offTopic).toHaveLength(2);
    expect(offTopic.map((c) => c.label)).toContain("Just Chatting: Recipes");
    expect(offTopic.map((c) => c.label)).toContain("Just Chatting: Movies");
  });
});
