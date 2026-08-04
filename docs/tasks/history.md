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

## T-0004 — Multi-subject syllabus catalogue foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done), T-0002 (done), T-0003 (done)

### Goal

Build safe, metadata-only multi-subject curriculum catalogue infrastructure so future
approved Cambridge syllabuses can be onboarded without code changes. Remove the hard-coded
`0610`/`5090` two-code restriction from Core validation and shared contracts, replacing it
with registry-backed validation. No content, questions, OCR, source ingestion, or public
copyrighted data is created.

### Scope

- New normalized, metadata-only tables `subjects` and `syllabuses` (+ immutable
  `syllabus_events` audit trail).
- Nullable FK path from `content_sources` to `syllabuses` (`catalogue_syllabus_id`), added
  non-destructively; existing audit tables and rows untouched.
- Registry-backed content-source syllabus validation (resolves a supplied code to a
  registered **active** catalogue syllabus; unknown/inactive/ambiguous → stable 400 before
  DB write; omitted → allowed).
- Authenticated Core catalogue endpoints: readers list/get active syllabuses & subjects;
  admin-only create/update. New least-privilege permissions `content_catalogue:read` and
  `content_catalogue:manage`; deny-by-default for unknown roles.
- Catalogue mutations audited with the verified Clerk subject, names-only (non-content)
  audit trail.
- Shared TypeScript contracts: `SyllabusCode` union → `string`; add catalogue types.
- Docs: D-0007, `docs/curriculum-catalogue.md`, local setup, handoff.

### Schema decisions

- `subjects`: `id` (UUID), `name` (TEXT, UNIQUE), timestamps. Normalized so board/subject
  are never free-text on requests.
- `syllabuses`: `id` (UUID, stable), `board`, `syllabus_code`, `subject_id` (FK → subjects),
  `qualification` (level, e.g. "Cambridge IGCSE" / "Cambridge O Level"), `track` (nullable,
  e.g. "Extended"), `display_name`, `curriculum_year` (nullable — only when explicitly
  known), `status` (`draft`|`active`|`retired`, default `draft`), timestamps.
- Uniqueness: **not** syllabus_code alone. `UNIQUE (board, syllabus_code, COALESCE(track,''))`
  so a NULL track cannot silently duplicate a `(board, code)` pair, and different boards may
  reuse a code.
- FK path: `content_sources.catalogue_syllabus_id UUID NULL REFERENCES syllabuses(id)`, added
  with `ADD COLUMN IF NOT EXISTS` (no data loss). Legacy `syllabus_code` TEXT column kept for
  the two seeded rows; its hard-coded `CHECK (... IN ('0610','5090'))` is dropped so the
  registry — not a fixed enum — is the authority.

### Migration path (backward compatibility)

- Existing seeded content_sources rows (migration 0003) keep their `syllabus_code` text and
  get `catalogue_syllabus_id = NULL`. They are **not** silently auto-mapped to catalogue
  syllabuses; a human links them in a later task. Documented in `docs/curriculum-catalogue.md`.
- New/updated content sources that supply a code have it resolved to an active catalogue
  syllabus, and the resolved `catalogue_syllabus_id` FK is stored alongside the code.

### Seed limits

Seeded exactly:
- Cambridge International / Biology / Cambridge IGCSE / Extended / 0610
- Cambridge International / Biology / Cambridge O Level / (no track) / 5090

Both seeded `active` (they are the D-0004 first vertical slice). Subject `Biology` seeded.
No invented year/edition, objective, assessment rule, rights claim, or extra syllabus.

### Assumptions

- "Readers" for the catalogue = roles that already hold content-source read (editor,
  reviewer, admin). Learner and unknown roles are denied, matching the existing
  content-source matrix and the T-0004 test requirement "learner/unknown denied". A
  public/learner-facing subject picker is explicitly out of scope.
- Seeded biology syllabuses are `active` (not `draft`) because they are the confirmed first
  slice and must resolve for existing 0610/5090 content-source metadata.
- Syllabus association on a content-source request is carried by the existing `syllabusCode`
  field, resolved server-side against the registry. Requests never carry free-text
  board/subject/level. Code resolution requires exactly one active match; zero or multiple →
  treated as unknown (stable 400).
- `curriculum_year` for the two seeds is left NULL: the provenance register documents
  "2026–2028" exam years for the source PDFs, but the task forbids inferring a curriculum
  year/edition into the catalogue without explicit human confirmation.

### Review-fix pass

- Fixed: subject creation now writes an immutable `subject_events` audit row (migration 0009,
  `subject_id`/`event_type`/`actor_id`/`event_time`/`changed_fields`), same transaction as the
  subject insert, append-only via trigger. Migration-seeded Biology subject is documented as an
  explicit bootstrap exception (no event, no invented actor).
- Fixed: catalogue HTTP handlers no longer return raw `err.Error()` for infrastructure failures;
  all map to one stable `internal_error` message. Safe domain errors unchanged.

### Acceptance checks

- Catalogue tables/audit trail created; content-source FK added non-destructively. — met
- Registry-backed syllabus validation replaces hard-coded 0610/5090 enum. — met
- Least-privilege catalogue read/manage permissions; learner/unknown denied. — met
- Catalogue mutations audited with verified Clerk subject, names-only fields. — met
- Only the two D-0004 biology syllabuses seeded, both active; no invented curriculum data. — met
- Internal errors never leak raw infrastructure text. — met
- Relevant tests pass. — met, see release validation below

### Constraints

- Never stage/alter/move/delete untracked user files: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, source ingestion, or copyrighted data. Catalogue holds
  curriculum metadata only.
- No web UI / subject picker. No AI catalogue authority (Core is the single authority).

### Open questions / blockers

- None blocking. Linking the two seeded `pending` content_sources rows to catalogue
  syllabuses is deferred to a future task (requires human provenance confirmation).

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `git diff --check` | Pass — clean |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./... -v` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource green; 9 integration tests skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `postgres-test` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 9 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — 6 catalogue + 3 content-source integration tests |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |

### Handoff

`docs/handoffs/T-0004.md`

## T-0005 — Provenance-confirmed catalogue linking

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0001 (done), T-0002 (done), T-0003 (done), T-0004 (done)

### Goal

Let an editor/admin explicitly confirm and establish a missing or stale
`content_sources.catalogue_syllabus_id` link through the existing authenticated `PATCH
/content-sources/{id}` flow. Migration 0008 (T-0004) intentionally left every existing pending
source's catalogue FK `NULL` rather than auto-mapping it; the pre-T-0005 `syllabusCode` diff
logic compared only the free-text code, so re-supplying the already-stored code was always `400
no_changes` — no safe application-layer path existed to complete the link, and no direct
database workaround is acceptable (bypasses auth/audit).

### Scope

- `services/core/internal/contentsource/postgres_store.go`: `Update` now diffs `syllabusCode`
  text and `catalogue_syllabus_id` independently instead of treating the FK as a byproduct of a
  text-code change.
  - Different code → `syllabus_code` + `catalogue_syllabus_id` updated, audited
    `["syllabusCode"]`.
  - Same code, missing/stale FK → `catalogue_syllabus_id` updated alone, audited
    `["catalogueSyllabusId"]` (never claims `syllabusCode` changed).
  - Same code, matching FK → unchanged `400 no_changes`, no write, no event.
- `handlers_test.go`'s `memoryStore.Update` mirrors the same split-diff logic so handler-level
  tests exercise identical semantics without a live database.
- Request/response shape unchanged: caller still supplies only `syllabusCode`; Core resolves the
  active catalogue syllabus server-side (unchanged registry-backed validation from T-0004).
  Unknown/inactive/ambiguous code still fails `400 unknown_syllabus` before any write.
- No new endpoint, no caller-supplied catalogue ID, no auto-linking (migration/startup/read/
  create/background job), no approval/OCR/ingestion/rights/status change.
- Docs: new `docs/provenance-catalogue-linking.md`; D-0007 updated in place (see
  `docs/decisions.md`); `docs/curriculum-catalogue.md` migration-path section cross-referenced.

### Acceptance checks

- Existing code + `NULL` FK → same-code PATCH links; verified actor; `changed_fields` only
  `catalogueSyllabusId`. — met
- Existing code + wrong FK → safe relink. — met
- Existing code + matching FK → `no_changes`; no event; `updated_at` unchanged. — met
- Different code → code/FK update, correct audit (`["syllabusCode"]`). — met
- Unknown/inactive/ambiguous code → `400` before write; no event. — met
- Omitted code unchanged. — met
- Non-pending source → `409`; no event. — met
- Learner/unknown denied. — met
- Disposable `sidus-test` integration proves FK/audit/immutability. — met
- All existing tests remain green. — met

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, ingestion, or copyrighted data. No rights/status change.
- No silent auto-mapping of the two seeded 0610/5090 content-source rows — a human still calls
  the API per source (documented in `docs/provenance-catalogue-linking.md`).

### Open questions / blockers

- None blocking. The two seeded pending content-source rows remain unlinked
  (`catalogueSyllabusId: null`) until a human editor/admin calls the documented `PATCH` for each
  — this task added the safe path; it does not itself perform the linking. See "Manual steps
  for the two seeded sources" in `docs/provenance-catalogue-linking.md`.

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./... -v` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource green; 10 integration tests skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `sidus-test-postgres-test-1` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 9 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — 6 catalogue + 4 content-source integration tests, incl. `TestPostgresStore_Integration_ProvenanceCatalogueLinking` |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Pass — clean |

### Handoff

`docs/handoffs/T-0005.md`

## T-0006 — Curriculum-map foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0001 (done), T-0004 (done), T-0005 (done)

### Goal

Build metadata-only curriculum-map infrastructure for all subjects: topic maps, learning
objectives, practical skills, and assessment rules, scoped under the existing curriculum
catalogue. No syllabus text, objective wording, topic labels, assessment text, questions, mark
schemes, PDFs, extracts, diagrams, OCR output, or derivative content is created. No map data is
seeded.

### Scope

- New tables `curriculum_map_nodes` and `curriculum_map_events` (migrations 0010–0011),
  package `services/core/internal/curriculummap`.
- Node fields: stable id, syllabus FK, optional parent-node FK (same syllabus, no cycles),
  `nodeKind` (topic/objective/practical_skill/assessment_rule), unique-per-syllabus
  `nodeCode`, editorial `label` placeholder, lifecycle `status`
  (draft/verified/retired), required approved content-source FK, optional `sourceLocator`,
  timestamps.
- Server-side source gate (Core is sole authority): every create, and every update that
  changes `contentSourceId`, verifies the referenced `content_sources` row exists, is
  `approved`, and its `catalogue_syllabus_id` matches the node's syllabus. Unknown/
  unapproved/unlinked/mismatched → stable `400` before any write.
- New least-privilege permissions `curriculum_map:read` (editor/reviewer/admin, verified-only
  reads), `curriculum_map:create` (editor/reviewer/admin, draft create/PATCH),
  `curriculum_map:verify` (reviewer/admin, verify/retire). Learner/unknown denied.
- Core API: list/get verified nodes by syllabus, create draft, PATCH draft, verify, retire.
  Strict JSON (no caller actor field); every route Clerk-protected; no raw internal error text.
- Shared TypeScript contracts, D-0008, `docs/curriculum-map.md`, `docs/local-setup.md` update.
- No AI-service map authority; AI service untouched.

### Review findings (fixed on top of `b1677cb`, commit `a05c523`)

Four findings from independent review, all fixed in this task's scope. No change to the
authority model, schema purpose, role matrix, approval/provenance rules, public content
restrictions, Clerk setup, or the "no map data seeded" stance. See D-0008 "Update (T-0006
review)".

1. **PATCH strict-JSON bypass** — unknown fields including `actorId`/`reviewerId`/`syllabusId`/
   `status` silently accepted via a `map[string]json.RawMessage` decode where
   `DisallowUnknownFields` has no effect. Fixed with an explicit six-field allowlist plus a
   strict struct decode.
2. **Source gate only ran when `contentSourceId` changed** — now re-validated on every node
   write (update/verify/retire), before any mutation.
3. **`GET /curriculum-map/nodes` returned an empty `200` for an unknown/inactive syllabus** —
   now `400 unknown_syllabus`.
4. **Ancestor walk was not actually row-locked** — every traversed ancestor is now locked `FOR
   UPDATE` in the writing transaction.

### Schema decisions

- Parent-same-syllabus and no-cycle enforcement are done at the **application layer** (Go, same
  transaction, row-locked ancestor walk) rather than a DB trigger — keeps `invalid_parent` error
  mapping under Core's control instead of parsing trigger-raised text. See D-0008
  "Alternatives".
- `syllabusId` is immutable after node creation (not PATCHable).
- `nodeCode` uniqueness is a DB unique index (`syllabus_id`, `node_code`); required
  `content_source_id` is a `NOT NULL` FK.
- `curriculum_map_events` mirrors `syllabus_events`/`content_source_events`: append-only,
  `BEFORE UPDATE OR DELETE` trigger, verified-Clerk-subject actor, changed-field-names only.

### Acceptance checks

- Metadata-only tables + immutable audit trail created; migrations idempotent on rerun. — met
- Source gate rejects unapproved/unlinked/mismatched/unknown sources before any write. — met
- Parent-same-syllabus and no-cycle enforced before any write. — met
- Duplicate node code per syllabus rejected. — met
- Lifecycle transitions (draft→verified, draft/verified→retired) enforced; invalid transitions
  rejected. — met
- Role matrix: learner/unknown denied; editor read+draft only; reviewer adds verify/retire;
  admin all. — met
- Strict JSON (unknown fields / trailing JSON rejected on both POST and PATCH); no caller
  actor/reviewer field. — met
- Source gate re-validated on every node write, not only on `contentSourceId` change. — met
- List rejects unknown/inactive syllabus with `400 unknown_syllabus`; empty list only for a
  known active syllabus. — met
- Ancestor walk row-locks every traversed ancestor. — met
- No raw database/internal error text ever returned. — met
- No map data seeded; two seeded 0610/5090 sources remain pending/unlinked until a human
  completes rights approval + catalogue linking. — met
- Relevant tests pass. — met, see release validation below.

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, source ingestion, or copyrighted data. No manual PATCH performed
  on the two seeded 0610/5090 content sources (human action only).
- No web UI for curriculum-map authoring in this task. No AI curriculum-map authority.

### Open questions / blockers

- None blocking. Authoring actual map content (topic labels, objective wording, etc.) requires
  a future private, approved editorial workflow — out of scope here, this task shipped
  infrastructure only. Linking/approving the two seeded 0610/5090 content sources remains a
  human action (carried from T-0005), not performed by this task.
- Observation, not fixed: a malformed (non-UUID) node id on `GET`/`PATCH`/`verify`/`retire`
  still yields `500 internal_error` rather than `404 not_found` — pre-existing behaviour shared
  with the catalogue and content-source packages, not one of the four review findings. Worth a
  separate cross-package task.

### Release validation (final pass, 2026-08-04)

| Command | Result |
| --- | --- |
| `go build ./... && go vet ./... && gofmt -l .` (Docker `golang:1.22-alpine`) | Pass — gofmt flags only pre-existing unrelated `internal/auth/auth_test.go` and `main_test.go` |
| `go test ./...` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource, curriculummap all green |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `docker compose -f docker-compose.test.yml up -d` + `pg_isready` wait | Pass — `sidus-test-postgres-test-1` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 11 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — catalogue (6) + content-source (4) + curriculum-map (17, incl. the concurrency ancestor-lock test) |
| `go run ./cmd/migrate` rerun against the same `sidus-test` | Pass — idempotent, 0 migrations applied |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; 6 routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Pass — clean |

### Handoff

`docs/handoffs/T-0006.md`
