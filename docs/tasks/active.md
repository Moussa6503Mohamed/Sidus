# Active tasks

## T-0034 — Automated Sonnet written-response marking

**Status:** review
**Type:** AI marking contract + Core attempt integration + learner result UI + tests.

### Context

T-0029 records written, structured, and essay responses as `pending_review`. T-0033 provides a
strict, fail-closed Sonnet job boundary with fake-provider coverage. Sidus is AI-first: no human
reviewer path belongs in normal marking.

### Goal

Turn eligible pending written attempts into immutable, criterion-level AI marking outcomes through
the T-0033 quality gate, while remaining fully safe when no Anthropic API is configured.

### Scope

- Define Core/AI contract for an owned, version-pinned written-marking request and result.
- Add durable attempt marking lifecycle: pending AI, marking accepted, automatically withheld.
- Preserve original question/rubric revision and criterion-level trace; never overwrite a final
  accepted mark.
- Add fixed learner BFF/UI polling/result states. No answer, rubric, canonical key, provenance,
  or source data leaks before permitted result state.
- Tests use fake provider only. Real Sonnet provider/key/calls remain disabled.

### Allowed files

- `services/core/internal/learner/**`, `services/core/internal/question/**`, `services/core/main.go`,
  `services/core/migrations/0024_*`, Core tests.
- `services/ai/app/sonnet/**`, `services/ai/app/**`, AI tests/dependencies only if required.
- `apps/web/app/dashboard/{practice,exam}/**`, `apps/web/app/api/learner/**`,
  `apps/web/lib/learner/**`, web tests/styles.
- `packages/shared/src/contracts.ts`, `docs/assessment-sessions.md`,
  `docs/sonnet-written-marking.md`, `docs/decisions.md`, `docs/handoffs/T-0034.md`,
  `CLAUDE.md`, this task file.

### Forbidden

- Live Anthropic API/key/calls, PDF/OCR/extraction/rewrite, source-content reads, seeds, or
  question/rubric prose.
- Human reviewer queues or manual marking except future teacher-configured exception.
- Public serving/logging of learner answers, sources, canonical keys, or provenance.
- Protected/untracked files and unrelated refactors.

### Acceptance criteria

- Only an owned pending written attempt with a pinned verified canonical rubric can request AI
  marking; duplicate requests are idempotent.
- Strict schema/rubric/grounding/confidence gate controls acceptance; failures retry or withhold
  automatically with safe user-visible state, never fabricated marks.
- Accepted output persists once with immutable criterion marks/feedback, model/version/cost/
  confidence trace; later calls cannot replace it.
- Learner sees pending, accepted, withheld/retry-safe result states only for own attempts.
- No configured provider/API means no mark is fabricated and API path fails closed or remains
  pending by documented contract.

### Validation

- Core build/vet/unit plus fresh disposable migration/integration/rerun.
- AI fake-provider suite; no live network.
- Web tests/typecheck/build; shared strict TypeScript; both Compose configs; diff/secret audit.

### Review checklist

- Ownership, idempotency, race/final-state safety, revision pinning, lifecycle transitions,
  learner projection, no source/key leakage, no manual-review path, API-key-free behavior.

### Stop condition

Implementation committed, independent review approved, release validation passed, docs release
committed/pushed. Live Anthropic integration test remains explicitly deferred.

### Implementation plan (2026-08-12)

Assumptions (recorded, not blocking — none change scope/security/rights/cost):
- Core calls the AI service synchronously over HTTP for one written-attempt marking request,
  mirroring T-0033's own "submit + run synchronously" job route. Service-to-service auth is a new
  shared-bearer-token dependency in the AI service (`SIDUS_CORE_SERVICE_TOKEN`), separate from the
  learner-facing Clerk-session `/sonnet/jobs` routes — Core is not a Clerk principal.
- `learner_attempts` stays untouched (its `prevent_learner_attempt_pin_mutation` trigger already
  forbids any update once `status='submitted'`, by design — not relaxed, see
  `[[feedback_preserve_tested_security_contracts]]`). A new `written_marking_requests` table (one
  row per attempt, unique) carries the AI marking lifecycle instead; final result is joined in at
  read time.
- `explanation_version` (part of the canonical cache key) is a fixed adapter-side constant for
  this task (`"v1"`); no versioning UI exists yet.
- `prompt_content_ref` stays a pure opaque pointer (the attempt id) — no source/answer content
  ever crosses the Core→AI boundary, matching current system behavior (no real provider exists).

Plan:
1. AI service (`services/ai/app/sonnet/**`, `services/ai/app/**`): add
   `require_service_token` auth dependency (fail-closed when `SIDUS_CORE_SERVICE_TOKEN` unset);
   add `POST /sonnet/marking-jobs` + `GET /sonnet/marking-jobs/{id}` reusing the existing
   orchestrator/job store/provider, owner-scoped to a fixed service principal. Tests: fake
   provider only.
2. Core migration 0024: `written_marking_requests` + `written_marking_events` (additive,
   immutable-once-terminal, append-only events), scoped to one attempt each.
3. Core `internal/learner`: `SonnetMarker` interface + `HTTPSonnetMarker` (calls step 1) +
   fail-closed default; `MarkingStore` interface (sessions.go pattern) + Postgres impl
   (`RequestMarking` idempotent create-or-return, `GetMarking` read-only); handlers
   `POST/GET /learner/attempts/{id}/marking`, owner-scoped, mounted only when the store
   implements `MarkingStore` (matches `SessionStore` wiring). `Register()` gains a `marker`
   parameter; `main.go` wires it from `SIDUS_AI_SERVICE_URL`/`SIDUS_AI_SERVICE_TOKEN` env.
4. `packages/shared/src/contracts.ts`: learner-safe marking projection types.
5. Web: `lib/learner/core-proxy.ts` operations, `app/api/learner/attempts/[id]/marking/route.ts`,
   `app/dashboard/practice/api-client.ts`, minimal poll/request UI in `question-list.tsx` for
   `pending_review` written/structured/essay results.
6. Docs: `docs/sonnet-written-marking.md`, `docs/decisions.md` entry, `docs/handoffs/T-0034.md`,
   `CLAUDE.md` current-state update.
7. Tests: Go unit (store/handlers) + integration (disposable Postgres, ownership/idempotency/
   race/no-provider/withhold/accept/immutability), Python fake-provider route tests, web
   proxy/route tests. Full scoped validation per task's Validation section.
