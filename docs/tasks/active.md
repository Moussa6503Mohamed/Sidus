# Active tasks

## T-0015 — Learner-safe verified-question delivery foundation

**Status:** review
**Owner:** Claude Code agent
**Type:** code+tests (Core Go, web BFF, web UI, shared contracts, docs)
**Start commit:** `80e130abac01b999dbb87433b96d26a92cd86e54` (clean `main`)

**Review-fix update:** Two review findings fixed on top of `d60f6e8`: (1) the learner eligibility
query's canonical-rubric join was missing an `rv.question_id = q.id` ownership check; (2) added
`GET /learner/syllabuses` (reuses `learner_question:read`, no new permission) plus a matching BFF
operation and web `<select>` so `/dashboard/practice` no longer asks a learner to type a raw
syllabus UUID. See `docs/handoffs/T-0015.md` "Update (T-0015 review fix)" and D-0017's "Update
(T-0015 review)" for full detail. Status stays `review` — not released, not pushed.

### Context

T-0007/T-0013/T-0014 built private editorial infrastructure for original questions and
versioned/canonical rubrics. No learner-facing surface has ever existed: every prior read
route (`GET /questions*`) requires an editorial permission (`question:read`) and returns the
full internal `Question`/`RubricVersion` shape, including status, canonical rubric id, rubric
JSON, and answer key. This task adds the first read-only surface a `learner` role may call.

### Goal

A learner can discover and read only safe, verified, grounded questions. The response can
never contain the answer key, rubric, canonical rubric id, reviewer/editor identity, audit
data, internal source metadata, or draft/retired content.

### Scope

1. **Core learner projection and routes** — new `services/core/internal/learner` package,
   dedicated `GET /learner/questions` and `GET /learner/questions/{id}` routes, new
   `learner_question:read` permission held by every recognized role (learner included).
   Editorial `/questions` routes are untouched.
2. **Web learner BFF** — new `apps/web/lib/learner/*` (separate `LearnerOperation` union from
   `EditorialOperation`) and `app/api/learner/questions/*` route handlers, reusing the T-0009
   fail-closed/sanitization contract.
3. **Minimal learner screen** — new authenticated `/dashboard/practice` page: syllabus/node
   input, loading/empty/error/access-denied/read states. Renders MCQ options as selectable but
   never submits, marks, reveals an answer, starts a timer, creates an attempt, or calls AI.
4. **Contracts, docs, tests** — shared learner projection types that structurally cannot carry
   sensitive fields, `docs/learner-question-delivery.md`, a decision record, and
   `docs/handoffs/T-0015.md`.

### Allowed files

- `services/core/internal/learner/**` (new)
- `services/core/internal/auth/auth.go`, `auth_test.go` (add permission only)
- `services/core/main.go` (register route)
- `packages/shared/src/contracts.ts`
- `apps/web/lib/learner/**` (new)
- `apps/web/app/api/learner/**` (new)
- `apps/web/app/dashboard/practice/**` (new)
- `apps/web/app/layout.tsx` (nav link only)
- `docs/learner-question-delivery.md`, `docs/decisions.md`, `docs/handoffs/T-0015.md`,
  `docs/tasks/active.md`, `docs/tasks/history.md`, `CLAUDE.md` (current-state bullet)

### Forbidden files

- Any editorial `question`/`curriculummap`/`contentsource`/`catalogue` route, store, or schema
  change beyond the new permission constant.
- `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx`, `.env.local`, any PDF/ZIP.
- No new SQL migration unless a genuine schema gap is found (none expected — the projection
  reads existing columns only).

### Plan-first questions / assumptions

- No learner-facing curriculum-catalogue/curriculum-map read route exists and none is added by
  this task (out of scope). The practice screen's "syllabus picker" / "node filter" are plain
  validated ID inputs, not a browsing UI — avoids widening catalogue/curriculum-map read
  permissions to the learner role, which the spec did not request.
- `GET /learner/questions/{id}` requires no `syllabusId`; an ineligible or nonexistent question
  returns the same `404` either way (no distinguishing signal).

### Acceptance criteria

- Learner projection JSON contains only: `id, syllabusId, curriculumMapNodeId, responseType,
  language, prompt, options, contentRevision`. No other key, ever, at any layer (Core, BFF,
  shared type, UI).
- A question is returned only when: status verified; canonical rubric exists, is verified, and
  matches current content revision; curriculum node is verified; node's source is approved and
  catalogue-linked to the question's syllabus — re-checked on every read, never cached.
- Unknown role denied at Core; learner/editor/reviewer/admin all succeed.
- Malformed/unknown syllabus or node id: stable 4xx, never 500.
- Web BFF: fixed allowlisted operations only, fail-closed config/token/redirect/5xx, no logging
  of token/body/Core URL/response.
- UI: zero network calls for denied role; no submit/mark/reveal/timer/attempt/AI action exists.

### Validation commands

- `npm --prefix apps/web run typecheck`
- `npm --prefix apps/web run test`
- `npm --prefix apps/web run build`
- `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts`
- Docker `golang:1.22-alpine`: `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`
- Disposable `sidus-test`: fresh `go run ./cmd/migrate`, rerun idempotency, `go test ./... -run
  Integration`, then `docker compose -f docker-compose.test.yml down -v`
- `python -m pytest` (services/ai — expected unchanged/green; no AI code touched)
- `git diff --check`
- Staged-content audit (no protected untracked file staged; no seeded/copied content)

### Review checklist

- No sensitive field reachable from the learner route at any layer.
- Grounding/canonical-rubric gate re-checked at read time (not trusted from question status
  alone).
- Editorial routes/permissions unchanged; role matrix additive only.
- BFF stays a closed operation union; no open proxy surface.

### Handoff requirements

`docs/handoffs/T-0015.md` with delivered summary, files changed, exact validation results,
commit hash, decisions/assumptions, open questions/blockers, protected-file status.

### Stop condition

Stop and mark `blocked` if any gate requires exposing rubric/answer-key data, if a schema
change turns out to be required, or if a learner-facing catalogue/curriculum-map browse
endpoint turns out to be required for the picker (would need explicit scope confirmation).
