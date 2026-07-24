# Active tasks

## T-0004 — Multi-subject syllabus catalogue foundation

**Status:** review
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

Seed exactly:
- Cambridge International / Biology / Cambridge IGCSE / Extended / 0610
- Cambridge International / Biology / Cambridge O Level / (no track) / 5090

Both seeded `active` (they are the D-0004 first vertical slice). Subject `Biology` seeded.
No invented year/edition, objective, assessment rule, rights claim, or extra syllabus.

### Assumptions

- "Readers" for the catalogue = roles that already hold content-source read (editor,
  reviewer, admin). Learner and unknown roles are denied, matching the existing
  content-source matrix and the T-0004 test requirement "learner/unknown denied". A
  public/learner-facing subject picker is explicitly out of scope (item 12).
- Seeded biology syllabuses are `active` (not `draft`) because they are the confirmed first
  slice and must resolve for existing 0610/5090 content-source metadata.
- Syllabus association on a content-source request is carried by the existing `syllabusCode`
  field, resolved server-side against the registry. Requests never carry free-text
  board/subject/level. Code resolution requires exactly one active match; zero or multiple →
  treated as unknown (stable 400).
- `curriculum_year` for the two seeds is left NULL: the provenance register documents
  "2026–2028" exam years for the source PDFs, but the task forbids inferring a curriculum
  year/edition into the catalogue without explicit human confirmation.

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

### Review-fix pass (status stays `review`)

- Fixed: subject creation now writes an immutable `subject_events` audit row (migration 0009,
  `subject_id`/`event_type`/`actor_id`/`event_time`/`changed_fields`), same transaction as the
  subject insert, append-only via trigger. Migration-seeded Biology subject is documented as an
  explicit bootstrap exception (no event, no invented actor).
- Fixed: catalogue HTTP handlers no longer return raw `err.Error()` for infrastructure failures;
  all map to one stable `internal_error` message. Safe domain errors unchanged.
- See `docs/handoffs/T-0004.md` "Review fixes (this pass)" for full detail and test list.

### Handoff

`docs/handoffs/T-0004.md` (on completion).
