# Active tasks

## T-0016 — Practice Mode MCQ attempts, deterministic marking, and verified feedback foundation

**Status:** review
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0015 (done / released)

### Context

T-0015 exposes eligible verified questions through learner-safe reads, but persists no learner
work and reveals no answers. Practice Mode now needs one-shot MCQ attempts marked only from the
explicit canonical rubric pinned to the question revision at attempt creation.

### Goal

Add strict editorial MCQ feedback, owned immutable learner attempts, atomic deterministic submit,
learner-safe BFF contracts, and Practice Mode feedback without widening into Exam Mode or AI.

### Task type

Code + tests; additive migration; UI; security boundary; documentation.

### Scope

- Extend immutable rubric JSON validation so every multiple-choice rubric has one non-blank
  `correctExplanation` and exact, complete per-current-wrong-option explanations; reject feedback
  on non-MCQ rubrics.
- Add idempotent attempt/event migration, learner attempt Core types/store/routes, ownership,
  eligibility-time pinning, and one-way atomic deterministic submission.
- Add separate fixed-operation learner BFF routes and Practice Mode submit/result UI.
- Add learner-safe shared types, decision/documentation updates, tests, validation, handoff, and
  one implementation commit. No push or release.

### Allowed files

- `services/core/migrations/0019_*`
- `services/core/internal/question/**`, `services/core/internal/learner/**`,
  `services/core/internal/auth/**`, `services/core/main.go`, `services/core/main_test.go`
- `packages/shared/src/contracts.ts`
- `apps/web/lib/learner/**`, `apps/web/app/api/learner/**`,
  `apps/web/app/dashboard/practice/**`
- `apps/web/app/dashboard/editorial/questions/**` (existing rubric authoring form/types/tests only)
- `docs/practice-mcq-marking.md`, `docs/decisions.md`, `docs/local-setup.md`, `CLAUDE.md`
- `docs/tasks/active.md`, `docs/handoffs/T-0016.md`

### Forbidden files

- `.claude/`, `.claude-flow/`, `.env.local`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx`
- PDFs, ZIPs, design bundles, source material, runtime data, seeds, copied/sample educational prose
- Existing migrations; editorial BFF; AI service behavior; deployment/release files

### Plan-first questions

- Can existing rubric rows remain valid while new MCQ rubric creation requires feedback? Yes:
  decoder stays response-aware and no data/backfill occurs.
- Can attempt creation reuse T-0015 eligibility without trusting learner projection state? Yes:
  one Core transaction rechecks gates and pins question revision plus explicit canonical version.
- Can submit safely expose feedback? Yes: decode only pinned rubric, mark under attempt row lock,
  persist once, then return explicit result projection.

### Non-goals

- No Exam Mode, timer, session/paper flow, AI marking, generated/fallback explanation,
  end-of-paper flow, rate limit, streak, analytics, adaptation, randomization, or cache.
- No editorial attempt administration, rubric fallback, new content, backfill, seed, or source use.
- No deterministic marking behavior for non-MCQ questions.

### Acceptance criteria

- Strict decoder rejects every incomplete, stale, foreign, duplicated, mis-cased, wrongly typed,
  duplicate-key, unknown-key, trailing-JSON, and non-MCQ-feedback shape.
- Attempt creation accepts recognized roles under owner-only rules and atomically pins exact eligible
  question revision, explicit canonical rubric version, and max marks.
- Strict submit rejects malformed/unknown/foreign/stale option IDs before mutation; only own open
  attempt can transition once; concurrent/replayed submits yield stable conflict.
- Result contains exactly approved learner-safe fields and verified pinned canonical feedback;
  pre-submit responses leak no answer/rubric/internal metadata.
- BFF validates route IDs/body before Clerk/Core, uses closed operations, fails closed, rejects
  redirects, and sanitizes upstream 5xx.
- Practice UI supports selection, explicit guarded submit, accessible selected/correct distinction,
  canonical feedback, and safe loading/error/retry/empty/denied states without answer reveal.
- Required regression and disposable database validations pass.

### Validation commands

- `docker compose -f docker-compose.yml config`
- `docker compose -f docker-compose.test.yml config`
- `npm --prefix apps/web run test`
- `npm --prefix apps/web run typecheck`
- `npm --prefix apps/web run build`
- `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts`
- Core: `gofmt`, `go build ./...`, `go vet ./...`, `go test ./...`
- Disposable `sidus-test`: fresh migrate, idempotent rerun, integration suite, `down -v`
- `python -m pytest -q` in `services/ai`
- `git diff --check`; staged-content and secret audit

### Security and privacy constraints

- Verified Clerk `sub` is sole owner identity; recognized role required; no caller-supplied actor.
- Core remains authority. Feedback comes only from pinned verified canonical rubric for pinned
  revision. No fallback, source derivation, raw rubric, token, URL, upstream body, or internals.
- Audit stores event names/changed field names only; no selected option or feedback prose.

### Review checklist

- [ ] Independent review required before `done`.
- [x] Migration additive/idempotent and historical rows valid.
- [x] Ownership, role matrix, atomicity, pinning, leakage, BFF, and UI tests present.
- [x] No protected/source/runtime files touched or staged; one implementation commit only.
- [x] Full requested validation recorded in handoff.

### Handoff requirements

Create `docs/handoffs/T-0016.md` with changed files, architectural/security decisions, exact test
results, commit hash, blockers, protected-file status, and reviewer focus.

### Assumptions / blockers

- No blocker. Existing T-0015 eligibility joins and T-0014 explicit canonical selection remain
  authoritative; implementation may share SQL predicates inside learner store but not editorial
  response types.

### Stop condition

Stop at `review` after implementation, validation, one commit, and handoff. Do not push, release,
deploy, mark `done`, or touch forbidden files.
