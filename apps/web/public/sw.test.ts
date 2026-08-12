import { describe, expect, it, vi } from "vitest";
import fs from "fs";
import path from "path";
import vm from "node:vm";

const swSource = fs.readFileSync(path.join(__dirname, "sw.js"), "utf-8");

function loadServiceWorker(origin = "https://sidus.example") {
  const listeners: Record<string, (event: unknown) => void> = {};
  const cachesMock = {
    open: vi.fn().mockResolvedValue({ addAll: vi.fn().mockResolvedValue(undefined), put: vi.fn().mockResolvedValue(undefined) }),
    match: vi.fn().mockResolvedValue(undefined),
    keys: vi.fn().mockResolvedValue([]),
    delete: vi.fn().mockResolvedValue(true),
  };
  const selfMock = {
    addEventListener: (type: string, handler: (event: unknown) => void) => { listeners[type] = handler; },
    skipWaiting: vi.fn(),
    clients: { claim: vi.fn() },
    location: { origin },
  };
  const context = { self: selfMock, caches: cachesMock, console, URL, Promise, fetch: vi.fn().mockResolvedValue({ ok: true, clone: () => ({}) }) };
  vm.createContext(context);
  vm.runInContext(swSource, context);
  return { listeners, cachesMock, selfMock };
}

function fetchEvent(url: string, init: { mode?: string; method?: string } = {}) {
  return {
    request: { url, mode: init.mode ?? "cors", method: init.method ?? "GET" },
    respondWith: vi.fn(),
    waitUntil: vi.fn(),
  };
}

describe("Service Worker", () => {
  it("guards same-origin before excluding /api and /dashboard from caching", () => {
    expect(swSource).toMatch(/url\.origin\s*!==\s*self\.location\.origin/);
    expect(swSource).toMatch(/url\.pathname\.startsWith\("\/api"\)/);
    expect(swSource).toMatch(/url\.pathname\.startsWith\("\/dashboard"\)/);
  });

  it("lets a cross-origin request pass through untouched", () => {
    const { listeners } = loadServiceWorker();
    const event = fetchEvent("https://cdn.example.com/font.woff2");
    listeners.fetch(event);
    expect(event.respondWith).not.toHaveBeenCalled();
  });

  it("lets a cross-origin navigate request pass through untouched", () => {
    const { listeners } = loadServiceWorker();
    const event = fetchEvent("https://accounts.example.com/sso", { mode: "navigate" });
    listeners.fetch(event);
    expect(event.respondWith).not.toHaveBeenCalled();
  });

  it("does not intercept same-origin /api requests", () => {
    const { listeners } = loadServiceWorker();
    const event = fetchEvent("https://sidus.example/api/learner/questions");
    listeners.fetch(event);
    expect(event.respondWith).not.toHaveBeenCalled();
  });

  it("does not intercept same-origin /dashboard requests", () => {
    const { listeners } = loadServiceWorker();
    const event = fetchEvent("https://sidus.example/dashboard/exam");
    listeners.fetch(event);
    expect(event.respondWith).not.toHaveBeenCalled();
  });

  it("responds for a same-origin navigate request to the app shell", () => {
    const { listeners } = loadServiceWorker();
    const event = fetchEvent("https://sidus.example/", { mode: "navigate" });
    listeners.fetch(event);
    expect(event.respondWith).toHaveBeenCalledTimes(1);
  });

  it("responds for a same-origin static asset via stale-while-revalidate", () => {
    const { listeners } = loadServiceWorker();
    const event = fetchEvent("https://sidus.example/_next/static/chunk.js");
    listeners.fetch(event);
    expect(event.respondWith).toHaveBeenCalledTimes(1);
  });
});
