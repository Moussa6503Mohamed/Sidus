# Active tasks

## T-0026 — Local Exam Mode MVP

**Status:** review
**Owner:** Codex
**Task type:** code+tests, UI/design, security
**Depends on:** T-0015/T-0016/T-0018 (done / released); T-0025 stays independently in review

**Review-fix update:** timer now finalizes latest in-memory answers even at expiry, confirmation
does not pause countdown, dialog focus behavior is covered, and results label answered-only marks.
Authenticated two-question browser journey is implemented but pending documented local web runtime.

### Context

Learner delivery and Practice Mode already provide live verified-question eligibility, safe
projections, deterministic one-question MCQ marking, and a closed learner BFF. Fastest local MVP
composes those existing routes in web state; it does not claim server-persisted sessions or an
authoritative timer.

### Goal

Deliver a usable local authenticated web-only Exam Mode MVP by composing existing learner routes,
without changing Core, PostgreSQL, shared contracts, or Practice Mode behavior.

### Scope

In:

- Add `/dashboard/exam` using existing learner syllabus/question reads and per-question attempt
  create/submit writes.
- Select syllabus, optional opaque curriculum-node ID, and requested question count; admit only
  eligible MCQs with options and require enough questions before starting.
- Hold order, answers, flags, navigation, countdown, and partial finalization progress in client
  memory. Label timer clearly as local MVP behavior.
- Show one question at a time with back/next, flag, answered/flagged progress, explicit submit
  confirmation, time-expiry submission, aggregate score, and per-question safe canonical feedback.
- Finalize sequentially. Reuse a created attempt after submit failure and skip completed results
  on retry, preventing duplicate attempt creation/submission within current page lifetime.
- Add focused pure-state/finalization, accessible component, and signed learner Playwright
  coverage. Preserve Practice unchanged.
- Update task review state, current-state note, and exact handoff; commit without push.

Out / non-goals:

- No Practice Mode behavior/schema/route replacement.
- No Core, schema, migration, shared-contract, server exam-session, authoritative timer, refresh
  resume, cross-device resume, or aggregate exam API change.
- No AI/LLM, OCR, ingestion, PDFs/books/papers, external source fetch, private-content access, or
  real educational fixture/seed wording.
- No free-response marking, question randomization/adaptive selection, pause/extra-time/admin exam
  management, analytics, scheduler/worker, public release, deployment, push, or T-0025 release.
- No new learner curriculum-map browse permission. Optional node remains an opaque ID forwarded
  only to existing eligible-question list route.

### Allowed files

- `apps/web/lib/learner/*`
- `apps/web/app/dashboard/exam/**`
- `apps/web/app/layout.tsx`
- `apps/web/e2e/editorial-to-practice.e2e.ts`, `apps/web/e2e/lib/synthetic.ts`, and matching tests
- `CLAUDE.md`, `docs/tasks/active.md`, `docs/handoffs/T-0026.md`

### Forbidden files

- `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx`, every env file, every PDF/ZIP
- `D:\Sidus-private-content` and all external/private source material
- T-0025 task/handoff/decision/status files except reading existing committed E2E code
- Production data, source records, question/rubric records, Clerk configuration, credentials
- AI service behavior and private editorial source/question wording
- Every `services/core/**` file, migration, and `packages/shared/src/contracts.ts`

### Plan-first questions

1. **Duration?** Fixed 30 minutes in client state. UI says "Local MVP timer" and warns it is not
   server-authoritative or refresh-persistent.
2. **Count?** Learner chooses 2–10. Existing list response order is retained; first requested
   eligible MCQs are used. If too few exist, exam does not start.
3. **When do answers reach server?** Only after confirmation or timer expiry. Each selected MCQ
   gets existing create-attempt then submit-attempt calls, sequentially.
4. **What about unanswered questions?** No attempt is created; review records unanswered/zero marks
   locally with no fabricated answer, feedback, or max marks. Aggregate max uses returned attempt
   max marks for answered questions only and labels unanswered count separately.
5. **How does retry work?** Runtime finalization state stores attempt IDs and completed results by
   question ID. Retry resumes first incomplete operation and never repeats completed submissions.
6. **Refresh behavior?** Client state is intentionally lost. This limitation is disclosed; durable
   exam sessions require later Core/schema work outside T-0026.

### Acceptance criteria

- Authenticated recognized-role learner can choose active syllabus, optional node ID, and 2–10
  questions; start only when enough eligible MCQs exist.
- One-at-a-time accessible question UI supports answer choice, back/next, flag, progress, and clear
  local countdown. No correctness or feedback appears before finalization completes.
- Confirmation or expiry triggers sequential existing attempt create/submit calls. Partial failure
  exposes retry; completed results and created attempts are not repeated.
- Final view shows aggregate awarded/max marks from Core results, correct and unanswered counts,
  and per-question selected/correct labels plus only returned safe feedback.
- Existing learner BFF remains closed, Clerk-protected, body-capped, redirect-refusing, and Core-5xx
  sanitizing. No source locator or private/editorial field appears on Exam surface.
- Practice Mode focused and full web tests pass unchanged.
- No seed/content/source/private-data access; E2E writes only runtime opaque synthetic strings.

### Validation commands

- `npm --prefix apps/web run test`
- `npm --prefix apps/web run typecheck`
- `npm --prefix apps/web run build`
- `npm --prefix apps/web run e2e -- --project=denial-learner` against existing authenticated local harness
- `npm --prefix apps/web exec playwright test -- --list`
- `git diff --check`
- Core/schema unchanged diff check; staged allowlist; forbidden-content, source-locator, credential,
  private-path, and generated-artifact scans.

### Review checklist

- Independent reviewer checks client-only limitations/copy, timer/autosubmit behavior, partial retry
  state, duplicate-submission prevention, accessibility, no early feedback, Practice isolation,
  BFF allowlist/body caps, and content/secret audit.
- Status may become `review` after implementation, validation, handoff, and commit. Never `done`
  without independent approval.

### Handoff requirements

- `docs/handoffs/T-0026.md` records exact commit, files, absence of Core/schema/contracts changes,
  every command/result, E2E environment/result, assumptions, gaps, protected-file audit, and
  reviewer focus. Update `CLAUDE.md` current state. Preserve T-0025 `review` status verbatim.

### Stop condition

Stop after own findings are fixed, requested validation passes (or exact external blocker is
recorded), implementation is committed but not pushed, task status is `review`, and handoff names
remaining gaps. Do not release, deploy, push, merge, or mark done.

### Blockers

None at start. Authenticated browser E2E depends on existing local stack/web server/private Clerk
captures documented by T-0025; absence or staleness is an external validation blocker, not grounds
for auth bypass.

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
