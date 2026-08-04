# Provenance-confirmed catalogue linking

T-0005. Explains how a human editor/admin confirms and repairs the link between a pending
`content_sources` row and its curriculum-catalogue syllabus (`catalogue_syllabus_id`), and what
that confirmation does and does not mean. See [D-0007](decisions.md) and
[curriculum-catalogue.md](curriculum-catalogue.md) for the catalogue itself.

## What "confirmation" proves — and what it does not

A successful link-only `PATCH` proves exactly one thing: **this content source's `syllabusCode`
text resolves, at confirmation time, to this catalogue syllabus record.** That is a structural
association, decided by a human with `content_catalogue`-adjacent authority (editor/reviewer/
admin — the same roles that already hold content-source update permission).

It does **not**:

- Approve rights. Approval is the separate `POST /content-sources/{id}/approve` flow and still
  requires every field in `RequiredApprovalFields` (owner, title, sourceUrl, sourceHash,
  licenceReference, permittedUse, allowedAudience).
- Ingest, extract, or store any source material. No OCR, no content, no questions.
- Prove the syllabus is current. `curriculum_year` is a separate, optional catalogue field
  (never inferred — see D-0007); linking a source to a syllabus record makes no claim about
  edition/year currency.
- Change the source's lifecycle status. The source stays `pending` before and after.

## The problem this closes

Migration 0008 added `content_sources.catalogue_syllabus_id` non-destructively and deliberately
left every existing row's FK `NULL` — it does not auto-map legacy rows to catalogue syllabuses
(see "Migration path" in `curriculum-catalogue.md`). Before T-0005, the existing `PATCH
/content-sources/{id}` `syllabusCode` diff logic compared only the free-text code: if a caller
re-supplied the *same* `syllabusCode` the row already had, the request was treated as making no
change (`400 no_changes`), even though `catalogue_syllabus_id` was still `NULL`. There was no
safe application-layer path to establish or repair that FK — only a direct database write, which
is not an auditable, permissioned action.

## How linking works now

The request body is unchanged: a caller still supplies only `syllabusCode` (never a catalogue ID
directly). Core resolves the *active* catalogue syllabus for that code server-side, exactly as it
already does for `POST`/`PATCH` `syllabusCode` handling (unknown/inactive/ambiguous code → `400
unknown_syllabus`, before any write).

`PATCH /content-sources/{id}` with a supplied `syllabusCode` now has three outcomes, decided by
comparing the **stored text code** and the **stored catalogue FK** against what the code
resolves to:

| Stored `syllabus_code` | Stored `catalogue_syllabus_id` | Result |
| --- | --- | --- |
| different from supplied code | (any) | `syllabus_code` **and** `catalogue_syllabus_id` updated; audited as `changed_fields: ["syllabusCode"]` |
| same as supplied code | `NULL` or resolves to a different catalogue syllabus | `catalogue_syllabus_id` updated alone (link/relink); audited as `changed_fields: ["catalogueSyllabusId"]` — `syllabusCode` is never claimed as changed |
| same as supplied code | already resolves to the same catalogue syllabus | `400 no_changes`; no write, no event, `updated_at` unchanged |

The distinction matters for audit accuracy: a link-only confirmation is not a syllabus-code edit,
so the immutable `content_source_events` row it produces must say so.

## Confirmation rules

- **Never automatic.** No migration, startup routine, background job, read endpoint, or the
  `POST /content-sources` create path performs this linking. It only happens through an explicit
  authenticated `PATCH` that a human sends.
- **Permission.** Same as any other pending-source `PATCH`: editor, reviewer, or admin
  (`content-source:update`, from the existing role matrix — see `docs/curriculum-catalogue.md` §
  Roles for the parallel catalogue-read/manage matrix). Learner and unknown roles are denied
  (`403`).
- **Actor.** The audit actor is always the verified Clerk session subject (`auth.ClaimsFromContext`)
  — never a request-body field. This is unchanged from T-0002/T-0003.
- **Only `pending` sources.** `approved`/`rejected`/`expired` sources return `409
  invalid_status_transition` before any write, same as every other content-source `PATCH` field.
- **No silent auto-map.** A code must still resolve to exactly one *active* catalogue syllabus;
  unknown/inactive/ambiguous fails with `400 unknown_syllabus` before any source write or event,
  exactly as it already did for a genuine code change.

## Audit trail

A link-only confirmation writes exactly one immutable `content_source_events` row:

- `event_type = 'metadata_updated'` (unchanged; this is still a metadata update, just of the FK
  rather than the text code)
- `actor_id` = verified Clerk subject
- `changed_fields = ['catalogueSyllabusId']` — the FK field name only, never `syllabusCode`
  (which did not change) and never the old/new catalogue syllabus ID values themselves (this
  package never stores field *values* in the audit trail, per T-0002)

## Manual steps for the two seeded sources

The two content-source rows seeded by migration 0003 (`syllabus_code = '0610'` and `'5090'`)
still carry `catalogue_syllabus_id = NULL` after this task. T-0005 adds the safe API path; it
does **not** itself link those two rows, because that is exactly the kind of silent auto-mapping
D-0007 and this task explicitly refuse to do without a human decision.

To link them, an authenticated editor/reviewer/admin must call, for each row:

```
PATCH /content-sources/{id}
Authorization: Bearer <clerk session token>
Content-Type: application/json

{"syllabusCode": "0610"}   // or "5090" for the other seeded row
```

This resolves the code against the active catalogue syllabus (seeded in migration 0007) and
records the link-only audit event described above. Until a human performs this call, both seeded
rows remain unlinked (`catalogueSyllabusId: null` in their JSON representation) — this is
expected, not a bug, and is not itself a rights/provenance claim about the syllabus source.
