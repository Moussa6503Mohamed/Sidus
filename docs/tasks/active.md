# Active tasks

## T-0001 — Content rights/provenance gate

**Status:** ready  
**Owner:** Unassigned  
**Priority:** P0  
**Scope:** Add local PostgreSQL, rights/provenance schema, immutable reviews, shared contracts, core API checks, AI ingestion rejection, official-syllabus metadata seeds, tests, setup docs.

### Acceptance checks

- Docker Compose starts PostgreSQL.
- Empty database migration succeeds.
- Source states: `pending`, `approved`, `rejected`, `expired`.
- Approval requires owner, source URL, source hash, licence reference, permitted use, allowed audience, reviewer, decision date.
- AI ingestion rejects every non-approved source with auditable reason.
- Only source metadata for official 0610/5090 links is seeded.
- No source PDFs, extracts, diagrams, or derivative questions added.
- Relevant tests pass.

### Constraints

- Existing untracked root images/spreadsheets are user files. Never stage, alter, move, or delete them.
- No Redis in this task.
- No auth UI in this task.

### Open questions

- Review authorization model: temporary internal reviewer identifier now, or wait for full auth task? Default implementation may use a required string `reviewer_id`; no auth system assumed.

### Handoff

None yet.
