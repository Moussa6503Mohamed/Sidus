# Active tasks

## T-0036 — Teacher classes, consent, and assignments

**Status:** review
**Type:** Core authorization/data + learner/teacher UI + tests.

### Context

Sidus remains AI-first. Teachers may create scoped classes and assignments. Learners must actively
accept class membership and may revoke it. Teacher manual checking exists only when explicitly
selected on an assignment; it must never replace normal automated marking.

### Goal

Build private-beta class membership consent and assignment delivery without cross-class access,
silent enrollment, learner answer leaks, or a generic human-review workflow.

### Scope

- Teacher-owned classes, opaque invitations, explicit learner acceptance/revocation, append-only
  consent/audit events, and role-scoped roster views.
- Teacher assignments using verified eligible question/module selection and immutable assignment
  settings. Teacher chooses `automated` default or explicit `manual_teacher` checking.
- Learner assignment list/start/read states; teacher assignment/class management and aggregated
  assignment progress only. No raw learner answers, canonical keys, source/provenance, rubric
  descriptors, AI traces, or cross-class analytics.
- Fixed Core routes, closed BFF operations, shared contracts, white-primary Sidus UI.

### Allowed files

- `services/core/internal/teacher/**`, `services/core/internal/learner/**`,
  `services/core/internal/auth/**`, `services/core/main.go`, `services/core/migrations/0026_*`,
  Core tests.
- `apps/web/app/dashboard/**`, `apps/web/app/api/{learner,teacher}/**`,
  `apps/web/lib/{learner,teacher}/**`, web tests/styles.
- `packages/shared/src/contracts.ts`, `docs/teacher-classes-assignments.md`,
  `docs/decisions.md`, `docs/handoffs/T-0036.md`, `CLAUDE.md`, this task file.

### Forbidden

- Live AI/API calls, PDFs/OCR/extraction/source reads, content seeds, public invitation indexing,
  email/SMS delivery, parent/org views, billing, cross-class/global ranking, or generic manual
  reviewer queue.
- Raw learner answers, rubrics/canonical keys, sources/provenance, model traces/costs in teacher
  projections, browser logs, or events.
- Protected/untracked files and unrelated refactors.

### Acceptance criteria

- Invitation token is opaque, hashed at rest, bounded lifetime/use, no membership before explicit
  learner acceptance; revocation removes future teacher access without mutating history.
- Every class/assignment route is owner/member scoped; enumeration and cross-tenant access fail
  closed with stable non-leaking responses.
- Assignments snapshot immutable settings/question/module selection; automated is default;
  `manual_teacher` requires explicit teacher selection and returns a distinct pending-teacher
  state, never silently invokes/overrides AI marking.
- Teacher sees only assignment aggregates and consent state. Learner sees own assigned work only.
- UI has loading/empty/error/retry/access-denied/consent-revoked states, keyboard-safe forms and
  dialogs, white-primary tokens.

### Validation

- Core build/vet/unit; fresh disposable migration/integration/rerun including token/ownership/
  acceptance/revocation/assignment races.
- Web tests/typecheck/build; strict shared TypeScript; Compose/diff/secret audit.
- No live AI/API or private-source testing.

### Review checklist

- Token hashing/leakage, consent lifecycle, owner/member boundaries, revocation, immutable
  assignment snapshots, explicit manual mode only, teacher projection privacy, race/idempotency,
  BFF validation and UI a11y.

### Stop condition

Implementation committed, independent review approved, release validation passed, release docs
committed/pushed. Stop before T-0037.
