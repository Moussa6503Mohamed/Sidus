# Active tasks

## T-0030 — Private admin upload and AI review intake foundation

**Status:** review
**Type:** upload security + Core/AI contracts + admin UI + tests.

### Goal

Build private admin PDF intake and review-job foundation without exposing files or publishing
model output.

### Scope

- Admin-only upload request, private object metadata, quarantine state, strict size/MIME/signature
  validation, malware-scan adapter boundary, retention/deletion state, and immutable audit events.
- Core-to-AI review-job contract with approved-source gate, opaque file reference, structured
  Sonnet-ready review result schema, and manual publish/reject state.
- Editorial UI for upload/status/review queue. No learner route exposes uploaded files or review
  payloads.

### Boundaries

- No real API key, model call, OCR/text extraction, question generation, PDF content in Git, or
  automatic publication. Object storage uses local development placeholder only.
- No source content logged, copied into docs, or sent to Notion.

### Acceptance and validation

- Deny-by-default roles; upload limits, content signature, status transitions, retention and
  audit tests; fixed BFF routes; no file access from learner endpoints.
- Go/Python/web tests, strict contracts, disposable migrations, security review, handoff.

### Stop condition

Stop at review. Independent security review before release.
