# Active tasks

## T-0025 — Local authenticated browser E2E harness

**Status:** review
**Owner:** Codex (completion after Claude Code session limit)
**Depends on:** T-0023/T-0024 (local HTTPS Core stack + private dev TLS CA), T-0009–T-0018 (the
editorial/learner surfaces under test)

### Goal

Give the repo a committed, reproducible, **local-only** browser end-to-end harness that drives the
real signed-in UI → BFF → Core path against the isolated `sidus-local-import` HTTPS stack, so the
full editorial-to-learner journey can be proved without hand-clicking it, and without any auth
bypass or fake-auth mode.

### Scope

In:

- Add `@playwright/test` to `apps/web` devDependencies (dependency-risk check verdict: add).
- `apps/web/playwright.config.ts` (test run) and `apps/web/playwright.auth.config.ts`
  (interactive auth-capture run only).
- `apps/web/e2e/lib/*`: storage-state resolution/validation, synthetic runtime fixture
  generation, stack preflight. Unit-tested with the existing Vitest runner.
- `apps/web/e2e/*.e2e.ts`: the journey spec plus negative specs.
- Docs: `docs/e2e-harness.md`, `docs/decisions.md` D-0023, `docs/handoffs/T-0025.md`,
  `CLAUDE.md` current-state entry, `.gitignore` belt-and-braces entries.

Out (explicitly not in scope):

- Any auth bypass, test-only sign-in route, fake session, or role-injection mode.
- Any Clerk password, secret key, bearer token, cookie, or storage state inside this repo.
- Any change to production/dev Compose, Core, AI, shared contracts, schema, migrations, or
  business rules.
- Any seeded migration or runtime fixture content. Every record the harness creates is made at
  runtime through the real UI, from opaque nonces.
- Any `docker compose down` performed automatically — the user runs a manual test afterwards.

### Assumptions

1. The `sidus-local-import` stack (T-0023) is already up, Core reachable at `https://127.0.0.1`
   with the private CA at `D:\Sidus-private-content\local-dev\ca.pem`.
2. `apps/web` is run by the operator on **port 3001** with `SIDUS_CORE_API_URL=https://127.0.0.1`
   and `NODE_EXTRA_CA_CERTS` pointing at that CA, and the stack's `CLERK_AUTHORIZED_PARTIES`
   includes `http://localhost:3001`. The harness does **not** start either — it preflights both
   and fails with instructions.
3. The operator has Clerk users for the profiles they enable: `admin` (required),
   `learner`, `unknown` (a signed-in user with no/unrecognized `sidus_role`).

### Plan

1. Dependency-risk check on `@playwright/test`; only add if it passes. **Done — passes.**
2. `e2e/lib/storage-state.ts`: pure-Node resolver + validator. Refuses any path inside the repo,
   requires the private root, fails closed (typed `E2eAuthError`) on missing file, malformed
   JSON, no Clerk cookie, expired cookie, or missing/damaged/mismatched/stale/future capture-time
   sidecar. Never logs file contents.
3. `e2e/lib/synthetic.ts`: runtime-only opaque nonce fixtures. Every generated string is
   `<fixed harness prefix> <run nonce>` and nothing else — asserted by `assertSynthetic`, so no
   educational or source-derived text can ever reach a record.
4. `e2e/lib/preflight.ts`: web + Core health/TLS checks with actionable failures.
5. `e2e/auth.setup.ts` + `playwright.auth.config.ts`: interactive headed capture. The human types
   the credentials into the real Clerk UI; the harness only calls `context.storageState({ path })`
   into `D:\Sidus-private-content\e2e`.
6. `playwright.config.ts`: `baseURL` 3001, `workers: 1`, **all trace/video/screenshot artifacts
   off** and `outputDir` outside the repo (artifacts would otherwise persist session cookies).
   Four projects: `journey` (admin), `denial-learner`, `denial-unknown`, `fail-closed` (no state).
7. `editorial-to-practice.e2e.ts`: synthetic source → approve → node → verify → MCQ draft →
   rubric → verify rubric → canonical + verify question → Practice select → submit → feedback.
8. `denial.e2e.ts` / `fail-closed.e2e.ts`: learner + unknown editorial denial; signed-out
   redirect; missing/expired state fails closed.
9. Vitest unit tests for the storage-state validator and synthetic generator ("tool tests").
10. Docs + full validation sweep. One implementation commit. No push. Stop before review.

### Open questions

None blocking. Recorded assumptions above cover the operator-side prerequisites.

### Blockers

None.
