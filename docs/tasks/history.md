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

## T-0002 — Pending source metadata update

**Status:** done
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done)

### Goal

Let human curators complete metadata for `pending` content sources (including the seeded
0610/5090 syllabus rows) without bypassing rights approval. Add an auditable, append-only
event trail for every successful update.

### Scope

- `PATCH /content-sources/{id}` on Core API.
- Only `pending` sources may be updated; `approved`/`rejected`/`expired` return `409`.
- Updatable fields: `title`, `owner`, `sourceUrl`, `sourceHash`, `licenceReference`,
  `permittedUse`, `allowedAudience`, `syllabusCode`.
- Reject empty/whitespace-only values for any supplied field (`400`).
- `syllabusCode` must be `0610` or `5090` (`400`).
- `sourceUrl` must be an absolute HTTP/HTTPS URL (`400`).
- Duplicate `sourceUrl` returns `409`.
- Update `updated_at`.
- `actorId` required in request body (`400` if missing).

### Audit

- New append-only `content_source_events` table. `content_source_reviews` untouched.
- Every successful update records: source ID, `event_type = 'metadata_updated'`, actor ID,
  event time, changed field **names** only.
- Never store PDF content, extracted text, diagrams, or previous/new field **values**.
- Immutability enforced at DB level (trigger rejects UPDATE/DELETE).

### Rights rule

- PATCH never approves. Approval stays separate and still requires all existing approval
  fields. Seeded syllabus rows stay `pending` until a human supplies verified rights
  metadata and separately approves them. No invented owner/hash/licence/audience data.

### Shared contracts

- `packages/shared`: add PATCH request + source-event contracts. Align Go/Python where relevant.

### Assumptions / decisions

- **Superseded by review finding 1:** changed field names are now a real value-diff.
  `Update` fetches the current row, compares each supplied field against its stored value,
  and only applies/records fields that actually differ. A request where every supplied
  field matches the current value returns `400 no_changes` (no write, no event, `updated_at`
  untouched). A request with no updatable field supplied at all still returns
  `400 no_updatable_fields` (unchanged).
- Updatable fields use pointer/optional JSON so absent vs. present-null both mean "no
  change"; a present field set to `""`/whitespace is a validation error, not a clear.
- `actorId` is a free-text identifier (no auth system yet — mirrors T-0001 `reviewerId`).
- **Review finding 2:** integration tests against `content_source_events` /
  `content_source_reviews` can never clean up (both immutable at the DB level). Removed the
  silent-failure DELETE cleanup attempts; added `docker-compose.test.yml` (disposable
  `postgres-test` service, tmpfs-backed, separate from the dev `postgres` service/volume) and
  documented that `TEST_DATABASE_URL` must point at it, never at dev/prod.
- **Test-environment isolation fix.** `docker-compose.test.yml` had no explicit Compose
  project name, so it defaulted to the same project (`sidus`) as `docker-compose.yml`,
  risking `down -v` on the test file removing dev resources. Fixed: added `name: sidus-test`
  (and `name: sidus` to the dev file) so containers/networks/volumes are fully distinct;
  verified by running both stacks simultaneously and tearing down only the test one. Also
  corrected the file's header comment, which referenced a nonexistent Compose `migrate`
  service — migrations run via `go run ./cmd/migrate` through the `golang:1.22-alpine`
  image. See `docs/handoffs/T-0002.md` for exact commands.

### Acceptance checks

- Pending source update succeeds. — met (`TestUpdate_Success`)
- Whitespace-only supplied value returns `400`. — met (`TestUpdate_WhitespaceOnlyValue_Returns400`)
- Invalid syllabus code returns `400`. — met (`TestUpdate_InvalidSyllabusCode_Returns400`)
- Invalid/non-HTTP URL returns `400`. — met (`TestUpdate_InvalidSourceURL_Returns400`)
- Duplicate URL returns `409`. — met (`TestUpdate_DuplicateURL_Returns409`)
- Non-pending source update returns `409`. — met (`TestUpdate_NonPending_Returns409`)
- Missing actor ID returns `400`. — met (`TestUpdate_MissingActorID_Returns400`)
- Successful update creates an immutable event, listing only actually-changed fields. — met
  (`TestUpdate_CreatesImmutableEvent`, `TestUpdate_MixedSameAndNewValues_RecordsOnlyChangedFields`,
  live `TestPostgresStore_Integration_UpdateOnlyChangedFields`)
- All-same-value update returns `400 no_changes`, no event, `updated_at` unchanged. — met
  (`TestUpdate_AllSameValues_Returns400NoChanges`, `TestUpdate_NoChangeRequest_NoEventAndNoUpdatedAtChange`,
  live `TestPostgresStore_Integration_UpdateOnlyChangedFields`)
- Existing T-0001 tests remain green. — met
- Live PostgreSQL event-immutability integration test passes against a disposable DB. — met
  (`-run Integration`, all 3 integration tests pass against `docker-compose.test.yml`, DB
  destroyed after)

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `.claude/`, `.claude-flow/`.
- No Redis, auth UI, Exam Mode, or unrelated feature work.

### Open questions (carried forward, not blocking)

- Actor authorization model still deferred to a future auth task (carried from T-0001).

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `docker compose -f docker-compose.yml config` | Pass |
| `docker compose -f docker-compose.test.yml config` | Pass |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./... -v` (unit, Docker toolchain) | Pass — 28 tests pass, 3 integration skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `postgres-test` healthy |
| `go run ./cmd/migrate` against isolated `postgres-test` | Pass — 4 migrations applied |
| `go test ./internal/contentsource/... -run Integration -v` against `postgres-test` | Pass — all 3 integration tests pass |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 5 tests |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass — no type errors |
| `git diff --check` | Pass — no whitespace errors |

### Handoff

`docs/handoffs/T-0002.md`

## T-0003 — Clerk authentication and roles foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done), T-0002 (done)

### Goal

Clerk owns authentication; Sidus Core owns authorization. No user-controlled `actorId` or
`reviewerId`. Audit identity (event actor, review reviewer) derives only from the verified
Clerk session subject.

### Scope

**Web (`apps/web`, Next.js 16 / React 19):** `@clerk/nextjs@^7.6.0`, `ClerkProvider`,
`proxy.ts` middleware protecting `/dashboard(.*)`, Clerk sign-in/sign-up routes, protected
dashboard placeholder, signed-in/out home page.

**Core (`services/core`, Go):** `internal/auth` package (role/permission matrix,
`ParseRole` deny-by-default, `Protect` middleware — 401 missing/invalid token, 403 valid
token lacking permission) backed by the official `clerk-sdk-go/v2` (pinned v2.5.0 for Go
1.22 compatibility) with JWKS TTL caching (no Backend API call per request). Content-source
routes wrapped with required permissions; `actorId`/`reviewerId` removed from request
bodies — audit actor/reviewer come only from the verified subject. Routes mount only when
DB + Clerk are fully configured (fail closed).

**AI (`services/ai`, FastAPI):** `ClerkAuthenticator` (PyJWT `PyJWKClient`, RS256) +
`require_clerk_session` dependency; protected `/ingestion/status` foundation only — no
OCR/ingestion added; rights gate unchanged.

**Contracts/docs:** `packages/shared/src/contracts.ts` — actor/reviewer fields removed,
`SIDUS_ROLES`/`SidusRole`/`SIDUS_ROLE_CLAIM` added. New `docs/auth-setup.md`. `.env.example`
Clerk placeholders only. `docs/decisions.md` D-0006.

### Review follow-up (fail-open hardening)

Closed four fail-open gaps, all now fail closed:

- Core issuer mandatory — content-source routes do not mount without `CLERK_JWT_ISSUER`.
- Authorized parties never silently unrestricted — absent → dev-default local origin only;
  present-but-blank → invalid (Core routes unmounted; AI protected routes → 503).
- AI issuer mandatory — a configured JWKS URL cannot bypass issuer validation; unconfigured
  auth fails closed with a generic 503.
- Content-source bodies parsed strictly (`DisallowUnknownFields` + reject trailing JSON
  values after the first decoded value) — unknown fields (incl. legacy
  `actorId`/`reviewerId`) or a second concatenated JSON value return `400 invalid_json`;
  audit actor/reviewer stay the verified `sub` only.

### Acceptance checks

- Clerk authenticates; Core/AI verify JWT offline via JWKS, no per-request Backend API
  call. — met
- Role authorization from verified `sidus_role` claim; missing/unknown role denied. — met
- `401` missing/invalid token, `403` valid token lacking permission. — met
- `actorId`/`reviewerId` cannot be supplied in request bodies; audit actor/reviewer =
  verified subject. — met
- Content-source routes fail closed without full Clerk/DB configuration. — met
- No real Clerk keys committed/staged; `.env.example` placeholders only. — met
- Relevant tests pass. — met, see release validation below

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `.claude/`, `.claude-flow/`, root `.env.local`.
- No Clerk Dashboard actions performed by the agent (manual human steps below).

### Open questions / blockers

- None blocking. Manual Clerk Dashboard steps (human, before beta): create the Clerk app
  and store real keys only in gitignored `.env.local` files; add session claim
  `sidus_role`; manually set the first admin (`public_metadata.sidus_role = "admin"`);
  configure production domain/origins and set real `CLERK_JWT_ISSUER` /
  `CLERK_AUTHORIZED_PARTIES` / `CLERK_JWKS_URL`. Full detail in `docs/auth-setup.md`.

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; `/api/health` present |
| `go build ./... && go vet ./... && go test ./... -v` (Docker `golang:1.22-alpine`) | Pass — all unit tests green, 3 integration tests skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.test.yml config` / `-f docker-compose.yml config` | Pass / Pass |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `postgres-test` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` postgres | Pass — 4 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — 3 immutable-audit integration tests |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Clean |

### Handoff

`docs/handoffs/T-0003.md`
