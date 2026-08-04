# Curriculum catalogue

Metadata-only registry of subjects and syllabuses. It is the single authority for which
syllabuses exist, so future approved Cambridge syllabuses are onboarded as **data** (an admin
API call), not code changes. See decision [D-0007](decisions.md).

**The catalogue never stores source material, questions, objectives, assessment rules, or
rights claims.** It holds curriculum identity metadata only. Rights/provenance for any source
material stays in the separate `content_sources` gate (T-0001/T-0002) — see
[content-provenance-register.md](content-provenance-register.md).

## Schema

Tables live in `services/core/migrations` (0005–0008).

### `subjects`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `name` | TEXT, **UNIQUE** | e.g. `Biology`. Referenced by id, never free-text on requests |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### `syllabuses`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `board` | TEXT | e.g. `Cambridge International` |
| `syllabus_code` | TEXT | e.g. `0610`. **Not** assumed globally unique |
| `subject_id` | UUID FK → `subjects` | subject relation |
| `qualification` | TEXT | qualification/level, e.g. `Cambridge IGCSE`, `Cambridge O Level` |
| `track` | TEXT NULL | tier/route, e.g. `Extended`; null when not applicable |
| `display_name` | TEXT | human label |
| `curriculum_year` | TEXT NULL | year/edition, **only when explicitly known — never inferred** |
| `status` | TEXT | `draft` \| `active` \| `retired` (default `draft`) |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Safe uniqueness:** `UNIQUE (board, syllabus_code, COALESCE(track,''))`. A syllabus code is
not globally unique; identity is the `(board, code, track)` triple. `COALESCE(track,'')`
collapses NULL and `''` so a null-track row cannot silently duplicate a `(board, code)` pair,
while different boards may reuse a code.

### `syllabus_events` (immutable audit)

Append-only trail of catalogue mutations: `syllabus_id`, `event_type`
(`syllabus_created`/`syllabus_updated`), `actor_id` (the **verified Clerk subject**),
`event_time`, `changed_fields` (names only). A `BEFORE UPDATE OR DELETE` trigger rejects any
mutation. Never stores field values or source material.

### `subject_events` (immutable audit)

Append-only trail of subject creation: `subject_id`, `event_type` (`subject_created`),
`actor_id` (the **verified Clerk subject**), `event_time`, `changed_fields` (names only, e.g.
`["name"]` — never the submitted value). A `BEFORE UPDATE OR DELETE` trigger rejects any
mutation. Subject creation and its audit event are written in one transaction: if the audit
insert fails, the subject creation rolls back (migration 0009).

**Seed exception:** the `Biology` subject seeded by migration 0007 is inserted with a plain SQL
`INSERT`, not through the `CreateSubject` application path used by the API. It is bootstrap
metadata for the D-0004 first vertical slice, not a human actor's action, so it intentionally
carries **no** `subject_events` row and no invented actor id.

### `content_sources.catalogue_syllabus_id`

Nullable FK → `syllabuses(id)`, added non-destructively (`ADD COLUMN IF NOT EXISTS`). Links a
content source to its catalogue syllabus. The legacy free-text `syllabus_code` column and its
data are retained; its old hard-coded `CHECK (... IN ('0610','5090'))` is dropped — the
registry, not a fixed enum, is now the authority.

## API

All endpoints require a Clerk session bearer token (see [auth-setup.md](auth-setup.md)).

| Method & path | Permission | Roles | Notes |
| --- | --- | --- | --- |
| `GET /catalogue/subjects` | `content_catalogue:read` | editor, reviewer, admin | list subjects |
| `POST /catalogue/subjects` | `content_catalogue:manage` | admin | `{ name }` |
| `GET /catalogue/syllabuses` | `content_catalogue:read` | editor, reviewer, admin | **active only** |
| `GET /catalogue/syllabuses/{id}` | `content_catalogue:read` | editor, reviewer, admin | active only (404 if not active) |
| `POST /catalogue/syllabuses` | `content_catalogue:manage` | admin | create |
| `PATCH /catalogue/syllabuses/{id}` | `content_catalogue:manage` | admin | change metadata/status |

`401` missing/invalid token; `403` valid token lacking permission; `400` validation
(`missing_required_fields`, `invalid_status`, `unknown_subject`, `blank_fields`,
`no_updatable_fields`, `no_changes`, `invalid_json`); `409` duplicate
(`duplicate_subject`, `duplicate_syllabus`); `404` not found. The audit actor is the verified
Clerk subject only — request bodies carry no actor field (strict decoder rejects unknown
fields).

`500 internal_error` (database, scan, transaction, or other infrastructure failure) always
returns the same stable, generic message. Raw driver/Go error text is never forwarded to a
client; only the safe, static domain-error messages above (`duplicate_subject`,
`duplicate_syllabus`, `unknown_subject`, `not_found`, `invalid_status`, `no_changes`, …) carry
any descriptive text.

## Roles

Least privilege (see `services/core/internal/auth`). Learner and unknown roles have **no**
catalogue access.

| Role | read | manage |
| --- | --- | --- |
| learner | — | — |
| editor | ✓ | — |
| reviewer | ✓ | — |
| admin | ✓ | ✓ |

## Content-source syllabus validation (registry-backed)

A content-source create/PATCH may supply `syllabusCode`. It is resolved server-side against
the catalogue:

- **omitted** → allowed; source stays unassociated (pending metadata).
- **resolves to exactly one active syllabus** → accepted; `catalogue_syllabus_id` FK is stored
  alongside the code.
- **unknown / inactive / ambiguous** (0 or >1 active matches) → stable `400 unknown_syllabus`
  **before any DB write**.

Requests never carry free-text board/subject/level — only the code, resolved server-side.

## Seed limits

Only the D-0004 first-vertical-slice biology syllabuses are seeded (migration 0007), both
`active`:

| Board | Subject | Qualification | Track | Code |
| --- | --- | --- | --- | --- |
| Cambridge International | Biology | Cambridge IGCSE | Extended | 0610 |
| Cambridge International | Biology | Cambridge O Level | — | 5090 |

No year/edition, objective, assessment rule, rights claim, or any other syllabus is seeded.

## Migration path / backward compatibility

- Existing seeded `content_sources` rows (migration 0003) keep their `syllabus_code` text and
  get `catalogue_syllabus_id = NULL`. They are **not** silently auto-mapped to catalogue
  syllabuses. A human links them via the authenticated `PATCH /content-sources/{id}` flow — see
  [provenance-catalogue-linking.md](provenance-catalogue-linking.md) (T-0005) for the exact
  request, permission, and audit rules.
- New/updated content sources that supply a code have it resolved to an active catalogue
  syllabus and store the FK. A `PATCH` that re-supplies the *same* stored code but whose
  `catalogue_syllabus_id` is missing or stale is a link-only provenance confirmation, not a
  code change — see [provenance-catalogue-linking.md](provenance-catalogue-linking.md).

## Onboarding checklist — adding a future syllabus

No code change is required. An **admin** calls the catalogue API. Every field below is a
**human-required input** — nothing is inferred from filenames or the copyrighted inventory:

1. **Approved official syllabus source** — confirmed, rights-cleared official syllabus
   reference (provenance approval recorded in the rights gate; the catalogue itself stores no
   source material).
2. **Board** — e.g. `Cambridge International`.
3. **Syllabus code** — e.g. `0620`.
4. **Subject** — create via `POST /catalogue/subjects` if new, then use its id.
5. **Qualification / level** — e.g. `Cambridge IGCSE`.
6. **Track** — e.g. `Extended`, if applicable (else omit).
7. **Current year / edition** — only if explicitly known; otherwise leave unset.
8. **Provenance approval** — human confirmation that the syllabus is cleared for onboarding.

Create as `draft`, then `PATCH … {"status":"active"}` once confirmed. Retire with
`{"status":"retired"}`; retired/draft syllabuses are hidden from readers and do not resolve for
new content-source associations.
