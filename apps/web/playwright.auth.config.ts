import path from "node:path";
import { defineConfig } from "@playwright/test";
import { privateStateDir } from "./e2e/lib/storage-state";
import { webBaseUrl } from "./e2e/lib/preflight";

/**
 * Interactive authentication capture only (T-0025) — `npm --prefix apps/web run e2e:auth`.
 *
 * Deliberately a separate config from `playwright.config.ts`: the capture run opens a headed
 * browser and blocks for up to ten minutes waiting for a human to sign in, which must never
 * happen inside a normal `npm run e2e`.
 *
 * Artifacts are off and `outputDir` is outside the repository for the same reason as in the test
 * config: a Playwright trace, video, or error-context snapshot of a signed-in page embeds live
 * session cookies, and this is the run where a session definitely exists.
 */
export default defineConfig({
  testDir: "./e2e",
  testMatch: /auth\.setup\.ts$/,
  outputDir: path.join(privateStateDir(), "auth-artifacts"),
  workers: 1,
  reporter: [["list"]],
  use: {
    baseURL: webBaseUrl(),
    headless: false,
    trace: "off",
    video: "off",
    screenshot: "off",
    ignoreHTTPSErrors: false,
  },
});
