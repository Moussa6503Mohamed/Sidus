import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { expect, test } from "@playwright/test";
import {
  assertPrivateRoot,
  isE2eProfile,
  privateStateDir,
  storageStateMetaPath,
  storageStatePath,
  type E2eProfile,
} from "./lib/storage-state";

/**
 * Interactive authentication capture (T-0025). Run only via `playwright.auth.config.ts`
 * (`npm --prefix apps/web run e2e:auth`) — it is deliberately not part of the test config, so a
 * normal harness run can never open a browser and wait for a human.
 *
 * A real Clerk sign-in happens here, performed by the human operator in a real browser window.
 * This file never reads, stores, types, or transmits a password, an email address, a Clerk
 * secret key, or a bearer token; it does not know what was typed. Its entire job is to call
 * `context.storageState()` afterwards and write the resulting cookies to a private directory
 * outside this repository. Nothing about the captured state is printed.
 */

const SIGN_IN_TIMEOUT_MS = 10 * 60 * 1000;

function resolveProfile(): E2eProfile {
  const raw = process.env.SIDUS_E2E_PROFILE?.trim();
  if (!raw) {
    throw new Error(
      'SIDUS_E2E_PROFILE is not set. Choose which Clerk user you are about to sign in as:\n' +
        '  $env:SIDUS_E2E_PROFILE = "admin"   # or "learner", or "unknown"\n' +
        "  npm --prefix apps/web run e2e:auth",
    );
  }
  if (!isE2eProfile(raw)) {
    throw new Error(`SIDUS_E2E_PROFILE="${raw}" is not a known profile (admin, learner, unknown).`);
  }
  return raw;
}

test("capture browser storage state for one Clerk profile", async ({ page, context }) => {
  test.setTimeout(SIGN_IN_TIMEOUT_MS + 60_000);

  const profile = resolveProfile();
  const repoRoot = path.resolve(__dirname, "../../..");
  const dir = privateStateDir();
  assertPrivateRoot(dir, repoRoot, profile);
  mkdirSync(dir, { recursive: true });

  await page.goto("/sign-in");

  // The operator signs in by hand. No credential ever passes through this process.
  process.stdout.write(
    `\nSign in as the "${profile}" Clerk user in the browser window, then leave it alone.\n` +
      `Waiting up to ${SIGN_IN_TIMEOUT_MS / 60000} minutes for /dashboard…\n\n`,
  );
  await page.waitForURL(/\/dashboard(\/|$|\?)/, { timeout: SIGN_IN_TIMEOUT_MS });

  // Prove the session really survives a fresh server-rendered request before saving it: a
  // half-completed sign-in would otherwise be written out as if it were usable.
  await page.goto("/dashboard");
  await expect(page.getByRole("heading", { name: "Dashboard", level: 1 })).toBeVisible();

  await context.storageState({ path: storageStatePath(profile, dir) });
  writeFileSync(
    storageStateMetaPath(profile, dir),
    `${JSON.stringify({ profile, savedAtSeconds: Math.floor(Date.now() / 1000) }, null, 2)}\n`,
    "utf8",
  );

  // Profile name only — never the state, the cookies, or any part of the session.
  process.stdout.write(`\nStorage state saved for profile "${profile}".\n`);
});
