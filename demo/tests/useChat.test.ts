import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useChat } from "../src/useChat";
import * as chatModule from "../src/chat";

vi.mock("../src/chat", () => ({
  chatReply: vi.fn(),
}));

describe("useChat", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("starts with empty messages", () => {
    const { result } = renderHook(() => useChat());
    expect(result.current.messages).toEqual([]);
    expect(result.current.loading).toBe(false);
  });

  it("sendMessage adds user + assistant messages", async () => {
    vi.mocked(chatModule.chatReply).mockResolvedValue("I can help!");

    const { result } = renderHook(() => useChat());

    await act(async () => {
      await result.current.sendMessage("Hello");
    });

    expect(result.current.messages).toHaveLength(2);
    expect(result.current.messages[0]).toEqual({
      role: "user",
      content: "Hello",
    });
    expect(result.current.messages[1]).toEqual({
      role: "assistant",
      content: "I can help!",
    });
  });

  it("loadConversation replaces messages", () => {
    const { result } = renderHook(() => useChat());

    const msgs = [
      { role: "user" as const, content: "Hi" },
      { role: "assistant" as const, content: "Hello!" },
    ];

    act(() => {
      result.current.loadConversation(msgs);
    });

    expect(result.current.messages).toEqual(msgs);
  });

  it("reset clears messages", async () => {
    vi.mocked(chatModule.chatReply).mockResolvedValue("Reply");

    const { result } = renderHook(() => useChat());

    await act(async () => {
      await result.current.sendMessage("Hi");
    });
    expect(result.current.messages).toHaveLength(2);

    act(() => {
      result.current.reset();
    });
    expect(result.current.messages).toEqual([]);
  });

  it("sets loading while waiting for reply", async () => {
    let resolveChat: (v: string) => void;
    vi.mocked(chatModule.chatReply).mockImplementation(
      () => new Promise((r) => { resolveChat = r; }),
    );

    const { result } = renderHook(() => useChat());

    let sendPromise: Promise<void>;
    act(() => {
      sendPromise = result.current.sendMessage("Hi");
    });

    expect(result.current.loading).toBe(true);

    await act(async () => {
      resolveChat!("Reply");
      await sendPromise!;
    });

    expect(result.current.loading).toBe(false);
  });
});
