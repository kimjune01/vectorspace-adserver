import { describe, it, expect } from "vitest";
import { themes, type Theme } from "../src/themes";

describe("themes", () => {
  it("has exactly 2 themes", () => {
    expect(Object.keys(themes)).toHaveLength(2);
  });

  it("has HealthChat AI theme keyed by pub-1", () => {
    const t = themes["pub-1"];
    expect(t).toBeDefined();
    expect(t.name).toBe("HealthChat AI");
    expect(t.publisherId).toBe("pub-1");
    expect(t.greeting).toBeTruthy();
    expect(t.primary).toMatch(/^#/);
    expect(t.bg).toMatch(/^#/);
  });

  it("has MindfulBot theme keyed by pub-2", () => {
    const t = themes["pub-2"];
    expect(t).toBeDefined();
    expect(t.name).toBe("MindfulBot");
    expect(t.publisherId).toBe("pub-2");
    expect(t.greeting).toBeTruthy();
    expect(t.primary).toMatch(/^#/);
    expect(t.bg).toMatch(/^#/);
  });

  it("each theme has required fields", () => {
    for (const t of Object.values(themes)) {
      expect(t).toHaveProperty("name");
      expect(t).toHaveProperty("publisherId");
      expect(t).toHaveProperty("greeting");
      expect(t).toHaveProperty("primary");
      expect(t).toHaveProperty("bg");
      expect(t).toHaveProperty("userBubble");
      expect(t).toHaveProperty("assistantBubble");
    }
  });
});
