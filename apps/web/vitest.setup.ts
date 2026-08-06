import "@testing-library/jest-dom/vitest";

// Node >=22's experimental global `localStorage` can win over jsdom's own per-window Storage
// implementation (jsdom cannot redefine an already-present global), leaving `window.localStorage`
// present but missing methods like `.clear()`/`.removeItem()`. If that happened, install a plain
// in-memory Storage so tests (e.g. ThemeToggle persistence) see a real, working implementation
// regardless of the host Node version.
function needsLocalStorageShim(): boolean {
  try {
    return typeof window === "undefined" || typeof window.localStorage?.clear !== "function";
  } catch {
    return true;
  }
}

if (needsLocalStorageShim()) {
  const store = new Map<string, string>();
  const memoryStorage: Storage = {
    get length() {
      return store.size;
    },
    clear: () => store.clear(),
    getItem: (key: string) => (store.has(key) ? store.get(key)! : null),
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key);
    },
    setItem: (key: string, value: string) => {
      store.set(key, String(value));
    },
  };
  Object.defineProperty(globalThis, "localStorage", { value: memoryStorage, configurable: true, writable: true });
}
