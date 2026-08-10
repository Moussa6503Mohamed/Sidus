import path from "node:path";
import { preflight } from "./lib/preflight";
import { requireStorageStatePath, requestedProfiles } from "./lib/storage-state";

/**
 * Fail-closed gate that runs once before any browser opens (T-0025).
 *
 * Two classes of silent failure are what this exists to prevent:
 *  - a run against a stack that is down, misconfigured, or serving a TLS chain Node will not
 *    verify — which would otherwise surface as an inscrutable timeout deep in a journey;
 *  - a run whose captured Clerk session is absent or stale, which must abort rather than quietly
 *    proceed unauthenticated and report a green "denied" result that proves nothing.
 *
 * Every profile the run asks for must have valid state. A profile is dropped only by explicitly
 * narrowing `SIDUS_E2E_PROFILES`, never by a file happening to be missing.
 */
export default async function globalSetup(): Promise<void> {
  try {
    await preflight();

    const repoRoot = path.resolve(__dirname, "../../..");
    for (const profile of requestedProfiles()) {
      // Throws E2eAuthError with capture instructions. The path is returned but not logged, and
      // the file's contents are never read into any message.
      requireStorageStatePath(profile, repoRoot);
    }
  } catch (error) {
    // Every failure reaching here is a precondition the operator must fix, and its message is
    // already the instruction, so print that alone rather than rethrowing a stack trace over it.
    // The run still exits non-zero. (On Windows/Node this teardown also emits a cosmetic libuv
    // "Assertion failed: !(handle->flags & UV_HANDLE_CLOSING)" line afterwards — noise from
    // Playwright's own shutdown, not a second failure. See docs/e2e-harness.md.)
    process.stderr.write(`\n${error instanceof Error ? error.message : String(error)}\n\n`);
    process.exit(1);
  }
}
