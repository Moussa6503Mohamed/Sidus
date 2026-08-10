import path from "node:path";
import { defineConfig, type Project } from "@playwright/test";
import { webBaseUrl } from "./e2e/lib/preflight";
import {
  privateStateDir,
  requestedProfiles,
  storageStatePath,
  type E2eProfile,
} from "./e2e/lib/storage-state";

/**
 * Local-only authenticated browser E2E harness (T-0025). See docs/e2e-harness.md.
 *
 * Runs against the operator's own `apps/web` dev server on port 3001 and the isolated
 * `sidus-local-import` HTTPS Core stack. It starts nothing and stops nothing — `globalSetup`
 * only checks that both are up, so the stack is still running for manual testing afterwards.
 *
 * Two security-shaped choices are load-bearing here, not stylistic:
 *
 *  1. Every artifact stream is off and `outputDir` points outside the repository. A Playwright
 *     trace, video, screenshot, or error-context snapshot of a signed-in page captures live
 *     Clerk session cookies; with artifacts on, a failing run would silently deposit credential
 *     material into `apps/web/test-results/`.
 *  2. `ignoreHTTPSErrors` stays false. Core is reached over real TLS with the private dev CA
 *     trusted through `NODE_EXTRA_CA_CERTS`; the harness never weakens verification to make a
 *     certificate problem go away.
 */

const STATE_DIR = privateStateDir();

/** One authenticated project per profile the run asks for. `globalSetup` has already proven each
 * one's state exists and is current, so a project here can never fall back to anonymous. */
function authenticatedProject(
  profile: E2eProfile,
  name: string,
  testMatch: RegExp,
): Project {
  return {
    name,
    testMatch,
    use: { storageState: storageStatePath(profile, STATE_DIR) },
  };
}

const PROFILES = requestedProfiles();

const projects: Project[] = [
  // No storage state at all: proves a signed-out browser is refused, which is the baseline the
  // authenticated results are only meaningful against.
  { name: "fail-closed", testMatch: /fail-closed\.e2e\.ts$/ },
];

if (PROFILES.includes("admin")) {
  projects.push(
    authenticatedProject("admin", "journey-admin", /editorial-to-practice\.e2e\.ts$/),
  );
}
if (PROFILES.includes("learner")) {
  projects.push(authenticatedProject("learner", "denial-learner", /denial-learner\.e2e\.ts$/));
}
if (PROFILES.includes("unknown")) {
  projects.push(authenticatedProject("unknown", "denial-unknown", /denial-unknown\.e2e\.ts$/));
}

export default defineConfig({
  testDir: "./e2e",
  testMatch: /\.e2e\.ts$/,
  globalSetup: "./e2e/global-setup.ts",
  outputDir: path.join(STATE_DIR, "test-results"),
  // The journey mutates shared Core state through the real editorial write path; running it
  // beside anything else would make failures unattributable.
  workers: 1,
  fullyParallel: false,
  // A flaky-looking pass here would be worse than a failure: it hides a real defect in the
  // authenticated path. Never auto-retry.
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  forbidOnly: true,
  reporter: [["list"]],
  use: {
    baseURL: webBaseUrl(),
    trace: "off",
    video: "off",
    screenshot: "off",
    ignoreHTTPSErrors: false,
  },
  projects,
});
