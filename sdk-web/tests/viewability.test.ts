import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { observeViewability } from "../src/viewability.js";

// Mock IntersectionObserver since it doesn't exist in Node
class MockIntersectionObserver {
  private callback: IntersectionObserverCallback;
  static instances: MockIntersectionObserver[] = [];

  constructor(callback: IntersectionObserverCallback, _options?: IntersectionObserverInit) {
    this.callback = callback;
    MockIntersectionObserver.instances.push(this);
  }

  observe = vi.fn();
  disconnect = vi.fn();
  unobserve = vi.fn();
  takeRecords = vi.fn(() => [] as IntersectionObserverEntry[]);

  get root() { return null; }
  get rootMargin() { return "0px"; }
  get thresholds() { return [0, 0.5]; }

  /** Simulate an intersection change for testing. */
  trigger(ratio: number) {
    this.callback(
      [{ intersectionRatio: ratio } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    );
  }
}

describe("observeViewability", () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;
  const fakeEl = {} as HTMLElement;

  beforeEach(() => {
    vi.useFakeTimers();
    fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("ok", { status: 200 }),
    );
    MockIntersectionObserver.instances = [];
    vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("creates an IntersectionObserver and observes the element", () => {
    observeViewability(fakeEl, 42, "adv-1", {
      endpoint: "http://localhost:8080",
    });

    expect(MockIntersectionObserver.instances).toHaveLength(1);
    expect(MockIntersectionObserver.instances[0].observe).toHaveBeenCalledWith(
      fakeEl,
    );
  });

  it("fires viewable event after 50%+ visible for 1 second", async () => {
    observeViewability(fakeEl, 42, "adv-1", {
      endpoint: "http://localhost:8080",
      publisherId: "pub-1",
    });

    const observer = MockIntersectionObserver.instances[0];
    observer.trigger(0.6); // 60% visible

    // Not fired yet
    expect(fetchSpy).not.toHaveBeenCalled();

    // After 1 second
    vi.advanceTimersByTime(1000);

    // Wait for microtasks
    await vi.advanceTimersByTimeAsync(0);

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [url] = fetchSpy.mock.calls[0];
    expect(url).toBe("http://localhost:8080/event/viewable");
    expect(observer.disconnect).toHaveBeenCalled();
  });

  it("cancels timer when element goes below 50%", () => {
    observeViewability(fakeEl, 42, "adv-1", {
      endpoint: "http://localhost:8080",
    });

    const observer = MockIntersectionObserver.instances[0];
    observer.trigger(0.6); // visible
    vi.advanceTimersByTime(500); // half second
    observer.trigger(0.3); // goes below threshold
    vi.advanceTimersByTime(600); // more than 1s total

    expect(fetchSpy).not.toHaveBeenCalled(); // should not fire
  });

  it("returns a cleanup function", () => {
    const cleanup = observeViewability(fakeEl, 42, "adv-1", {
      endpoint: "http://localhost:8080",
    });

    expect(typeof cleanup).toBe("function");

    const observer = MockIntersectionObserver.instances[0];
    observer.trigger(0.6); // start timer
    cleanup(); // should disconnect

    expect(observer.disconnect).toHaveBeenCalled();
    vi.advanceTimersByTime(1500);
    expect(fetchSpy).not.toHaveBeenCalled(); // timer was cleared
  });

  it("only fires once even if threshold is crossed multiple times", async () => {
    observeViewability(fakeEl, 42, "adv-1", {
      endpoint: "http://localhost:8080",
    });

    const observer = MockIntersectionObserver.instances[0];
    observer.trigger(0.6);
    vi.advanceTimersByTime(1000);
    await vi.advanceTimersByTimeAsync(0);

    // First fire
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    // Trigger again — should not fire again
    observer.trigger(0.3);
    observer.trigger(0.6);
    vi.advanceTimersByTime(1000);
    await vi.advanceTimersByTimeAsync(0);

    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });
});
