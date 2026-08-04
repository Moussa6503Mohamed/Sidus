# Active tasks

## T-0007 — Original question and rubric foundation

**Status:** review
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0001 (done), T-0004 (done), T-0005 (done), T-0006 (done)

### Goal

Build private, metadata-and-code-only infrastructure for **original** questions and **versioned**
rubrics that future Exam Mode will use. Every question traces to exactly one **verified**
curriculum-map node whose approved content source still passes the T-0006 source gate.

**No question text, rubric text, syllabus text, mark schemes, past-paper content, PDFs,
extracted text, diagrams, OCR output, or derivative questions are created or seeded by this
task.** The public repository holds schema, code, contracts, docs, and tests only; question and
rubric content exists solely in a runtime database, written by a future **private editorial
workflow** that is out of scope here.

### Scope

- New tables `questions`, `question_rubric_versions`, `question_events` (migrations 0012–0014,
  plus additive 0015 for content revision), new package `services/core/internal/question`.
- Question fields: stable id, syllabus FK, curriculum-map-node FK, response type
  (`multiple_choice` | `short_answer` | `structured_response`), language, original question
  prompt/body, lifecycle status (`draft` | `verified` | `retired`), timestamps.
- Rubric version fields: stable id, question FK, immutable positive per-question version number,
  rubric structure JSONB (validation-safe schema), maximum marks, status (`draft` | `verified`),
  creator/reviewer verified Clerk subjects, timestamps.
- Question event fields: question id, event type covering create/update/verify/retire and
  rubric-version create/verify, verified Clerk subject, names-only changed fields, immutable
  trigger blocking `UPDATE`/`DELETE`. Never stores prompt or rubric values.
- Core API: list/get verified questions (by syllabus, optional node); create/PATCH draft
  question; create rubric version; list rubric versions (editorial roles only); verify rubric
  version; verify question; retire question.
- New least-privilege permissions `question:read`, `question:create`, `question:verify`,
  `question_rubric:read`. Learner/unknown denied.
- Shared TypeScript contracts, D-0009, `docs/question-rubric-model.md`, `docs/local-setup.md`,
  `CLAUDE.md`, handoff.

### Out of scope (explicitly not done)

- No AI generation, Anthropic calls, OCR, ingestion, or question derivation. The AI service is
  untouched.
- No human source-rights approval, catalogue linking, or curriculum-map authoring performed.
- No question, rubric, or map data seeded (no source currently passes the gate anyway).
- No web UI for question authoring.

### Schema decisions

- **Node link, not syllabus-only grounding.** `questions.curriculum_map_node_id` is a `NOT NULL`
  FK; `questions.syllabus_id` is also stored and must equal the node's syllabus, checked in the
  application layer on every write. Storing both keeps list-by-syllabus a single indexed read
  while the equality invariant is enforced by Core.
- **Verified-node + source gate re-run on every question write** (create, PATCH, verify, retire,
  rubric-version create, rubric-version verify), mirroring the T-0006 review fix: a node can be
  retired, or its source un-approved/unlinked/re-linked, after a question was created.
- **`syllabusId` is immutable** on a question (same reasoning as D-0008); re-point the node
  instead, which re-validates the syllabus match.
- **Rubric versions are append-only per question.** `UNIQUE (question_id, version)`, version
  allocated inside the write transaction under a row lock on the question, and a DB trigger
  rejects any `UPDATE` that changes `question_id`, `version`, `question_revision`, `rubric`, or
  `max_marks` — only `status`/`reviewed_by`/`updated_at` may change (verification).
- **A rubric version is bound to the question content it was reviewed against** (review fix 1,
  migration 0015). `questions.content_revision` starts at 1 and is incremented by exactly one per
  successful draft content update; every rubric version stores the revision current at its
  creation. A version counts towards question verification only while the two are equal. An edit
  stales older versions without deleting or downgrading them.
- **Rubric JSONB has a validation-safe schema** validated before any write: a non-empty
  `criteria` array of objects with a non-empty `id`, a positive integer `marks`, and an optional
  `descriptor`; criterion marks must sum exactly to `maxMarks`. Every key is matched **exactly and
  case-sensitively** at every level, and unknown keys, case variants, duplicate keys, wrongly-typed
  values, and trailing JSON are all rejected (review fix 2).
- **A question can only be verified when it has at least one verified rubric version for its
  current content revision**, checked inside the verifying transaction. No verified version at all
  is `409 missing_verified_rubric`; verified versions that all predate the current content are
  `409 missing_current_verified_rubric` (review fix 1).
- **Retired questions disappear from reader endpoints** (verified-only reads), mirroring the
  curriculum-map/catalogue reader pattern.

- **`GET /questions` validates the optional node filter** (review fix 3): unknown/malformed →
  `400 unknown_node`, a real node of another syllabus → `400 mismatched_node`, a valid matching
  node with no verified questions → `200` with empty `items`. Weaker than the grounding gate on
  purpose — a retired node filters to an empty list, not an error.

### Acceptance checks

- Migrations bootstrap on an empty database and are idempotent on rerun.
- Content revision increments exactly once per successful edit; never on `no_changes`, a rejected
  write, or a lifecycle transition.
- A question cannot be verified with only a rubric verified against an older revision; stale
  versions stay verified, immutable, and readable.
- Rubric JSON rejects case variants and duplicate keys at every level.
- The optional node filter on listing is validated.
- No seeded question or rubric text anywhere in the repository.
- Question syllabus must equal the mapped node's syllabus; draft/retired/missing nodes rejected.
- Verified node + source gate revalidated on every question write and verification.
- Question cannot be verified without a verified rubric version.
- Rubric versions immutable, unique, and monotonic per question.
- Lifecycle transitions enforced; invalid transitions rejected.
- Question events immutable; actor is the verified Clerk subject; names-only changed fields.
- Full role matrix: learner/unknown denied; editor read+draft+draft-rubric; reviewer adds
  verify/retire; admin all.
- Strict JSON: unknown fields, `actorId`/`reviewerId`, `syllabusId` change, lifecycle spoofing,
  and trailing JSON all rejected before any store call.
- No raw database/internal error text returned.
- Existing test suite stays green.

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- Do not push.

### Open questions / blockers

- None blocking. Recorded assumptions:
  - `language` is an opaque non-empty string (e.g. a BCP-47 tag). No language registry exists
    yet and inventing one is out of scope.
  - Rubric listing is gated by its own permission (`question_rubric:read`, editor/reviewer/admin)
    rather than reusing `question:read`, because draft rubric structure is more sensitive than a
    verified question.
  - No question can actually be created until a human approves and links a content source
    (T-0001/T-0005) and authors + verifies a curriculum-map node (T-0006) — carried forward, not
    performed here.

### Handoff

`docs/handoffs/T-0007.md`
