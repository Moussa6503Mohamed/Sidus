# Active tasks

## T-0014 — Explicit canonical rubric selection

**Status:** review
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0007 (done), T-0012 (done), T-0013 (done)

### Goal

Require reviewer/admin selection of one current verified rubric when verifying a question, persist
that rubric as the question's canonical version, and allow explicit repair of historical verified
questions whose canonical reference is null.

### Scope and plan

- Add one additive, rerunnable migration with nullable question-owned canonical rubric FK and safe
  index; preserve every existing value and perform no backfill.
- Update Core contracts, transactional stores, strict handlers, permissions, immutable names-only
  audit events, and unit/integration/migration coverage.
- Extend shared TypeScript contract plus fixed-operation editorial BFF and guarded route coverage.
- Add reviewer/admin rubric selection, confirmation, canonical marker, historical repair, and UI
  role/empty/error tests without adding learner delivery or content.
- Document invariant, data safety, API/workflow contract, decision, setup note, and handoff.
- Run full requested validation, leave task in `review`, and commit one T-0014 commit without push.

### Assumptions

- `canonical_rubric_version_id` references rubric row UUID `id`; request `rubricVersion` remains
  positive per-question version number because existing routes and UI expose that stable number.
- Historical verified questions with null canonical are repaired only by new endpoint. Existing
  non-null canonical cannot be replaced, matching no-automatic-replacement and immutable lifecycle.
- Existing editorial question/rubric reads may expose nullable canonical ID; no learner route exists.

### Open questions / blockers

None. User supplied approved architecture and acceptance criteria.

### Validation (2026-08-06)

- Web: 21 files / 197 tests pass; typecheck and production build pass with canonical BFF route.
- Go: build, vet, gofmt, full unit suite pass.
- PostgreSQL: fresh disposable migrate applies 18; runner rerun applies 0; direct 0018 rerun keeps
  historical canonical null; full integration suite passes; `sidus-test` torn down with volumes.
- Python: 18 tests pass (dependency/cache warnings only).
- Strict shared TypeScript, dev/test Compose configs, and `git diff --check` pass.
