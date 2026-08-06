# Active tasks

## T-0018 — Licensed-adaptation provenance for questions

**Status:** review
**Owner:** Codex
**Type:** code + tests + additive migration + UI + documentation

### Context

Sidus needs explicit question origin and metadata-only licensed-adaptation provenance while
preserving existing curriculum grounding, canonical rubric, learner delivery, and attempt
pinning guarantees. Licence facts are human-entered evidence and must never be inferred.

### Goal

Require every newly created question to choose `original` or `licensed_adaptation`; persist
immutable licensed provenance metadata; enforce live source rights/link gates at verification
and learner delivery; expose provenance only to editorial users.

### Scope

- Add idempotent schema for nullable historical question origin plus immutable licensed
  `question_provenance` and append-only names-only audit events. No backfill.
- Extend strict shared/editorial/Core create contracts and required validation.
- Make origin/provenance immutable after create.
- Re-check licensed source approval, catalogue syllabus link, and complete human-entered rights
  fields during question verification and every learner eligibility read.
- Extend editorial create UI with origin choice, approved-source picker, locator, and warning.
- Keep learner question, attempt, and result projections provenance-free.
- Document human approval workflow and architecture decision.

### Allowed files

- `services/core/migrations/0020*.sql`
- `services/core/internal/question/**`
- `services/core/internal/learner/**`
- Core route wiring/auth tests where required
- `packages/shared/src/contracts.ts`
- `apps/web/app/api/editorial/**`
- `apps/web/app/dashboard/editorial/questions/**`
- `apps/web/lib/editorial/**`
- Relevant web learner contract/leak tests only
- `docs/licensed-adaptation-provenance.md`, `docs/decisions.md`, `docs/local-setup.md`,
  `CLAUDE.md`, `docs/tasks/active.md`, `docs/handoffs/T-0018.md`

### Forbidden files

- `.claude/**`, `.claude-flow/**`, `.env.local`, images, spreadsheets, PDFs, ZIPs
- Source/coursebook/past-paper/mark-scheme material, extracted or transformed content
- Runtime data, seed data, generated licence facts, production/staging resources
- AI/OCR/ingestion services except existing invariant tests if strictly needed

### Non-goals

- No ingestion, upload, OCR, parsing, extraction, transformation, generation, or recreation.
- No source preview or source-content API.
- No historical backfill or automatic origin classification.
- No learner provenance surface, source fallback, or latest-source resolution.
- No release, deploy, merge, or push.

### Plan-first questions resolved

- Before state: questions have no origin/provenance columns; sources already hold human-entered
  rights metadata and catalogue link. Historical rows must remain unchanged.
- After state: new questions require explicit origin; licensed questions have exactly one
  immutable metadata-only provenance row. Existing rows stay null/no provenance.
- Clean and initialized databases: migration uses additive/idempotent DDL and creates no rows.
- Rollback: no destructive rollback; additive objects remain until separately reviewed removal.
- Rights/content safety: source facts are only referenced and checked, never synthesized or
  copied; locator is metadata only.

### Acceptance criteria

- Strict case-sensitive `originType` create validation; unknown/malformed/foreign IDs fail before
  mutation with stable safe errors.
- `original` rejects source id/locator; `licensed_adaptation` requires approved, syllabus-linked,
  rights-complete source plus non-blank locator.
- Origin/provenance cannot change through update or direct database mutation.
- Verification and learner delivery fail closed after source rejection, expiry, unlinking,
  syllabus mismatch, or incomplete rights.
- Existing node grounding, canonical rubric, current revision, attempt ownership/pinning, and
  deterministic marking remain intact.
- Editorial-only provenance types/read data remain separate from learner projections.
- BFF uses fixed allowlist, strict input, auth forwarding, redirect refusal, and sanitized 5xx.
- Editorial UI covers original/licensed selection, approved source picker, locator, warnings,
  loading/error/empty/access states; contains no upload/preview/OCR/AI path.
- No seed/data mutation, backfill, provenance leak, protected-file change, secret, or source
  material enters commit.

### Validation commands

- `npm --prefix apps/web run test`
- `npm --prefix apps/web run typecheck`
- `npm --prefix apps/web run build`
- strict shared TypeScript compilation used by web typecheck
- `gofmt` check, `go build ./...`, `go vet ./...`, `go test ./...`
- fresh disposable PostgreSQL migration, migration rerun, full Integration tests, teardown
- `python -m pytest` in `services/ai`
- `docker compose config` and `docker compose -f docker-compose.test.yml config`
- `git diff --check`; staged secret/content/protected-file audit

### Review checklist

- Migration proves idempotency and no backfill on pre-existing questions.
- Full role matrix and strict JSON/path/query tests pass.
- Database and application immutability both tested.
- Licensed gate regressions tested at verify, list/get delivery, and attempt creation.
- Learner question/attempt/result JSON contains no provenance/licence fields.
- Independent reviewer confirms rights boundary and no copyrighted content handling.

### Handoff requirements

- Create `docs/handoffs/T-0018.md` with exact changed files, commands/results, commit, blockers,
  migration teardown, and protected-file audit.
- One implementation commit only. Do not push.

### Assumptions / open questions / blockers

- Existing content-source rights fields define completeness; implementation will reuse exact
  source approval semantics rather than invent new licence facts.
- No blockers. Implementation and validation complete; independent review required.

### Stop condition

Stop at `review` after implementation, full validation, handoff, and one commit. Never mark
`done` without independent review approval; never release or push.
