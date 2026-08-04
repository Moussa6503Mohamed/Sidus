# Active tasks

## T-0006 — Curriculum-map foundation

**Status:** review
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

### Schema decisions

- Parent-same-syllabus and no-cycle enforcement are done at the **application layer** (Go, same
  transaction, row-locked ancestor walk) rather than a DB trigger — keeps `invalid_parent` error
  mapping under Core's control instead of parsing trigger-raised text. See D-0008
  "Alternatives".
- `syllabusId` is immutable after node creation (not PATCHable) — avoids re-validating an
  entire subtree's parent-syllabus invariant on a syllabus change.
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
- Strict JSON (unknown fields / trailing JSON rejected); no caller actor/reviewer field. — met
- No raw database/internal error text ever returned. — met
- No map data seeded; two seeded 0610/5090 sources remain pending/unlinked (unchanged from
  T-0005) until a human completes rights approval + catalogue linking. — met
- Relevant tests pass. — met, see handoff for exact results.

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, source ingestion, or copyrighted data. No manual PATCH performed
  on the two seeded 0610/5090 content sources (human action only, per task instructions).
- No web UI for curriculum-map authoring in this task. No AI curriculum-map authority.

### Open questions / blockers

- None blocking. Authoring actual map content (topic labels, objective wording, etc.) requires
  a future private, approved editorial workflow — out of scope here, this task ships
  infrastructure only. Linking/approving the two seeded 0610/5090 content sources remains a
  human action (carried from T-0005), not performed by this task.

### Handoff

`docs/handoffs/T-0006.md`
