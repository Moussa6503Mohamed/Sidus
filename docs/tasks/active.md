# Active tasks

## T-0008 — Cross-package API input hardening

**Status:** review
**Owner:** Claude Code agent
**Depends on:** T-0003 (done), T-0006 (done), T-0007 (done)

### Goal

Make the existing Core APIs in `contentsource`, `catalogue`, and `curriculummap` reject malformed
IDs and case-variant request fields consistently, closing the two cross-package gaps the T-0007
review flagged as carried-forward observations (see `docs/handoffs/T-0007.md` "Blockers / open
questions"), before any web editorial client is built.

### Scope

- `services/core/internal/contentsource`, `services/core/internal/catalogue`,
  `services/core/internal/curriculummap` only.
- Every JSON body handler in these packages: reject unknown fields, case-variant field names
  (e.g. `SyllabusCode`, `Label`, `SourceUrl`), `actorId`/`reviewerId`, and trailing JSON — all
  before any store call, with the existing stable `invalid_json` error.
- Every route with an `{id}` path parameter: a malformed (non-UUID) id now maps to the same
  stable not-found response as a missing id, never a generic `500 internal_error`.
- `question` package: unchanged (already closed both gaps in T-0007).

### Non-goals

- No business-rule, role, schema, migration, or source approval/linking changes.
- No UI, AI/OCR/ingestion, or source/PDF/text/diagram/question/rubric content work.
- No shared-helper extraction across packages (each package keeps its own `decodeStrict`,
  matching the existing per-package pattern already used for `question`).

### What changed

- `contentsource`, `catalogue`, `curriculummap` handlers: `decodeStrict` now takes an explicit
  case-sensitive field allowlist (decode into `map[string]json.RawMessage` first, reject any key
  outside the allowlist, then strict-decode into the destination struct) — mirrors the pattern
  `question` already used. Applied to every POST/PATCH body: content-source create/update/
  approve/reject; catalogue create-subject/create-syllabus/update-syllabus; curriculum-map
  create-node (update-node already had this pattern from the T-0006 review).
- `contentsource`, `catalogue` `postgres_store.go`: added `isInvalidTextRepresentation` (checks
  Postgres `22P02`) and used it alongside `sql.ErrNoRows` everywhere an `{id}` path parameter is
  looked up — `Get`/`Update`/`Approve`/`Reject` (contentsource), `GetSyllabus`/`UpdateSyllabus`
  (catalogue). `curriculummap` already had this helper (T-0006 review); `GetNode`, `UpdateNode`,
  and `transitionStatus` (verify/retire) now use it too.

### Acceptance checks

- Case-variant fields rejected for every affected create/PATCH body shape.
- `actorId`/`reviewerId`, unknown fields, trailing JSON values, and trailing junk rejected before
  any store call, in every affected package.
- Malformed (non-UUID) IDs on GET/PATCH/verify/retire/approve/reject map to the existing
  not-found response, never `500`.
- A valid UUID for a missing resource still returns the existing not-found response (unchanged).
- Full role matrix and all pre-existing behavior remain green.

### Validation

See `docs/handoffs/T-0008.md` for the full command/result table.

### Blockers / open questions

None.
