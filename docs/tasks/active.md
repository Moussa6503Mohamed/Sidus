# Active tasks

## T-0005 — Provenance-confirmed catalogue linking

**Status:** review
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
- Unknown/inactive/ambiguous code → `400` before write; no event. — met (existing
  `TestUpdate_UnknownSyllabusCode_Returns400`, unchanged resolver path)
- Omitted code unchanged. — met (existing generic update tests untouched)
- Non-pending source → `409`; no event. — met (existing `TestUpdate_NonPending_Returns409`,
  unchanged — status check runs before the diff)
- Learner/unknown denied. — met (new `TestUpdate_CatalogueLinkConfirmation_LearnerDenied`)
- Disposable `sidus-test` integration proves FK/audit/immutability. — met
  (`TestPostgresStore_Integration_ProvenanceCatalogueLinking`)
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
  — this task adds the safe path, it does not itself perform the linking.

### Handoff

`docs/handoffs/T-0005.md`
