import { describe, expect, it } from "vitest";
import fs from "fs";
import path from "path";

describe("Service Worker", () => {
  it("excludes /api and /dashboard from caching", () => {
    const swCode = fs.readFileSync(path.join(__dirname, "sw.js"), "utf-8");
    expect(swCode).toMatch(/url\.pathname\.startsWith\("\/api"\)/);
    expect(swCode).toMatch(/url\.pathname\.startsWith\("\/dashboard"\)/);
  });
});
