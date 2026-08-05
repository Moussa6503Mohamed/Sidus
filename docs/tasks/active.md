# Active tasks

## T-0010 — Private editorial curriculum-map workflow

**Status:** review
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0006 (done), T-0009 (done)

### Goal

Extend the T-0009 secure Next.js editorial workflow so staff can author, edit, review, and
retire curriculum-map nodes through the browser. Core remains the sole authority for rights/
source/syllabus/parent/cycle/lifecycle rules; this task adds a BFF and UI surface over the
existing T-0006 curriculum-map API, plus one tiny, essential Core read-contract widening (see
"Core contract change" below). No node/source data is created, seeded, verified, retired, or
altered automatically by this task.

### Scope

- Protected `/dashboard/editorial/curriculum` page in `apps/web`, gated to `editor`/`reviewer`/
  `admin` (server-side role check from the verified Clerk session; `learner`/unknown see an
  access-denied state and trigger zero API calls).
- Narrowly scoped Next.js route handlers (BFF) reusing `lib/editorial/core-proxy.ts`'s closed
  `EditorialOperation` union — no open proxy:
  - `GET /api/editorial/curriculum-map/nodes?syllabusId=...&status=...`
  - `GET /api/editorial/curriculum-map/nodes/[id]`
  - `POST /api/editorial/curriculum-map/nodes`
  - `PATCH /api/editorial/curriculum-map/nodes/[id]`
  - `POST /api/editorial/curriculum-map/nodes/[id]/verify`
  - `POST /api/editorial/curriculum-map/nodes/[id]/retire`
  - Reuses the existing `GET /api/editorial/syllabuses` and content-source routes; no
    duplication.
- Metadata-only node authoring UI: syllabus selector, node kind, node code, editorial label,
  optional parent node, approved linked content-source selector, optional source locator.
  Shows hierarchy/parent, lifecycle status, source link state. Loading/empty/error+retry/
  create/edit/verify-confirm/retire-confirm/disabled/mobile-safe states.
- Editors: list/create/edit draft nodes. Reviewers/admins: also verify/retire. Mirrors the
  T-0009 role-visibility pattern — Core stays the sole authorization authority; the web check
  only hides/shows controls.
- **Core contract change (tiny, essential — see "Plan-first questions" below):** widen
  `GET /curriculum-map/nodes` and `GET /curriculum-map/nodes/{id}` to return nodes of any
  lifecycle status to a `curriculum_map:read` holder (editor/reviewer/admin only — no other
  consumer of this route exists), with an optional `status` filter on the list endpoint. No
  schema/migration change; no new role/permission; no change to the write-side source/parent/
  cycle/lifecycle gates.
- No content ingestion, OCR, AI generation, or copyrighted material of any kind. No PDF,
  extract, diagram, syllabus text, past paper, mark scheme, question, or rubric content.
- No automatic approval, link, node verification, or lifecycle action performed by this task
  itself — all remain explicit human actions through the new UI.

### Plan-first questions (asked and answered before implementation)

**Q:** Core's GET list/get for curriculum-map nodes hardcode verified-only reads, an explicit,
twice-documented D-0008 decision with two passing tests. But `curriculum_map:read` is already
restricted to editor/reviewer/admin (no learner/public consumer exists), so verified-only reads
mean editors can never browse/reopen their own drafts and reviewers can never discover a draft
to verify through the UI — breaking the workflow this task exists to build. How to resolve?
**A (user, before implementation):** Widen GET to all statuses for `curriculum_map:read`
holders (list gains an optional `status` filter, defaulting to all statuses; get-by-id returns
any status). Update the two existing tests that assumed verified-only and document the change
as a D-0008 addendum.

### Acceptance criteria

- All six BFF routes exist, delegate only to `callCore`/`readSafeJsonBody`, export exactly
  their intended HTTP method(s), and validate `{id}`/`syllabusId`/`status` before any Core call.
- `/dashboard/editorial/curriculum` renders access-denied with zero API calls for learner/
  unknown; editors can list, create, and edit draft nodes; reviewers/admins can additionally
  verify/retire with an explicit confirmation step.
- Core's `curriculum_map` package tests (existing + new) pass, including the widened-read
  behavior and its explicit `status` filter/validation.
- Existing T-0009 source-workflow tests remain green.
- `apps/web` typecheck/build/vitest pass; Go build/vet/test pass in Docker; a disposable
  Postgres migrate+integration+teardown cycle passes; Python tests pass; `git diff --check`
  is clean.
- `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx`, `.env.local` are never
  staged or modified.

### Validation commands

```sh
npm --prefix apps/web run typecheck
npm --prefix apps/web run test
npm --prefix apps/web run build
docker run --rm -v "$(pwd)/services/core:/app" -w /app golang:1.22-alpine sh -c "go build ./... && go vet ./... && go test ./..."
# disposable Postgres (docker-compose.test.yml) migrate -> integration tests -> teardown
cd services/ai && python -m pytest
git diff --check
```

### Handoff

See `docs/handoffs/T-0010.md` (written at task completion) and `docs/editorial-curriculum-workflow.md`.
