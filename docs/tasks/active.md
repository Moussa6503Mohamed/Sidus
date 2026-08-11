# Active tasks

## T-0032 — Persistent assessment sessions

**Status:** review
**Type:** Core migration/API + learner BFF/UI + tests.

### Goal

Replace browser-only Exam/Test state with server-owned immutable assessment sessions, autosave,
reconnect, server deadline, and idempotent final submission.

### Scope

- Add session, item snapshot, response mutation, result, and immutable event records.
- Create/get/autosave/submit/result learner routes with strict bodies, ownership, idempotency, and
  existing mixed response validation.
- Use current eligible-question gate at creation; pin learner-safe question snapshot and rubric
  revision. Keep keys, rubrics, provenance, and feedback hidden before submission.
- Adapt Test/Exam UI for autosave, refresh/reconnect, server timer, resume, and safe retry.

### Boundaries

- No private source/PDF access, AI call, automatic question generation, teacher workflow, or
  source-content disclosure. Written answers remain `pending_review`.

### Acceptance and validation

- One open Exam per learner; immutable order/snapshot; conflict-safe autosave; duplicate request
  idempotency; atomic submit; final feedback only; deadline enforcement.
- Go migration/unit/integration, web tests/typecheck/build, authenticated E2E, Python, contracts,
  Compose, diff/security checks.

### Stop condition

Stop at review. Independent review and release before next task.
