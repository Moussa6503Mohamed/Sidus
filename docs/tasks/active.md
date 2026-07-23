# Active tasks

## T-0001 — Content rights/provenance gate

**Status:** review
**Owner:** Claude Code agent (2026-07-23)
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
- Seed rows for CAM-0610-2026 / CAM-5090-2026 leave `owner`, `source_hash`, `licence_reference`, `allowed_audience` null — provenance register does not document a rights holder legal name, file hash, or licence reference for these links, and this task must not guess them. Seeds stay `pending` and cannot pass approval until a human reviewer supplies those fields. `permitted_use` is seeded verbatim from the register's "Approved use" column since that value is already documented there.
- No update/edit endpoint is in scope (Core API list is create/get/list/approve/reject only). Rights fields must be supplied at creation time (all optional at create) or a source will stay blocked from approval permanently. Follow-up task needed if curators must patch fields after creation.
- `expired` is a valid `status` value in the schema/contracts for forward-compatibility but no endpoint transitions a source to `expired` in this task (not listed under Core API scope).

### Implementation plan

1. `docker-compose.yml` (root): local `postgres:16-alpine` service, env-overridable creds, healthcheck, named volume. `.env.example` documents vars.
2. `services/core/migrations/*.sql`: numbered, idempotent SQL migrations for `content_sources`, `content_source_reviews` (with an immutability trigger blocking UPDATE/DELETE), and a seed migration for the two syllabus sources.
3. `services/core/cmd/migrate`: small Go migration runner (tracks applied files in `schema_migrations`, safe to rerun).
4. `services/core/internal/contentsource`: model, `Store` interface, Postgres implementation (`lib/pq`), HTTP handlers wired into `main.go` alongside existing `/healthz`.
5. `packages/shared/src/contracts.ts`: add `ContentSource`, `ContentSourceReview`, request/response contract types shared by future web/service clients.
6. `services/ai/app`: ingestion gate (`guard_ingestion`) that rejects every non-`approved` source and logs source ID + reason; unit tests.
7. Tests: Go handler tests against an in-memory fake `Store` (no DB needed for `go test ./...`), optional Postgres-backed integration test gated on `TEST_DATABASE_URL` (skipped if unset); Python ingestion tests.
8. Docs: `docs/local-setup.md` compose instructions, `infra/README.md` update, handoff file.

### Handoff

`docs/handoffs/T-0001.md`
