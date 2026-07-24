# Task history

## T-0001 — Content rights/provenance gate

**Status:** done
**Owner:** Claude Code agent
**Priority:** P0
**Scope:** Add local PostgreSQL, rights/provenance schema, immutable reviews, shared contracts, core API checks, AI ingestion rejection, official-syllabus metadata seeds, tests, setup docs.

### Acceptance checks

- Docker Compose starts PostgreSQL. — met
- Empty database migration succeeds. — met
- Source states: `pending`, `approved`, `rejected`, `expired`. — met
- Approval requires owner, source URL, source hash, licence reference, permitted use, allowed audience, reviewer, decision date. — met
- AI ingestion rejects every non-approved source with auditable reason. — met
- Only source metadata for official 0610/5090 links is seeded. — met
- No source PDFs, extracts, diagrams, or derivative questions added. — met
- Relevant tests pass. — met, see handoff for exact results

### Constraints

- Existing untracked root images/spreadsheets are user files. Never staged, altered, moved, or deleted.
- No Redis in this task.
- No auth UI in this task.

### Open questions (carried forward, not blocking)

- Review authorization model: temporary internal reviewer identifier used now (`reviewer_id` required string); no auth system assumed. Revisit when full auth task lands.
- Seed rows for CAM-0610-2026 / CAM-5090-2026 leave `owner`, `source_hash`, `licence_reference`, `allowed_audience` null — provenance register does not document these. Seeds stay `pending` and cannot pass approval until a human reviewer supplies those fields.
- No update/edit endpoint is in scope (Core API list is create/get/list/approve/reject only). Rights fields must be supplied at creation time or a source stays blocked from approval permanently. Follow-up task needed for a `PATCH /content-sources/{id}` endpoint.
- `expired` is a valid `status` value in the schema/contracts for forward-compatibility but no endpoint transitions a source to `expired` in this task.

### Handoff

`docs/handoffs/T-0001.md`
