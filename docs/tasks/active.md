# Active tasks

## T-0002 — Pending source metadata update

**Status:** review
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
- No Redis, auth UI, Exam Mode, or unrelated feature work. Do not push.

### Open questions / blockers

- None blocking. Actor authorization model still deferred to a future auth task (carried
  from T-0001).

### Handoff

`docs/handoffs/T-0002.md` (created at completion).
