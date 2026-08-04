# Question and rubric model

Private infrastructure for **original** questions and **versioned** rubrics that a future Exam
Mode will use. Scoped under the [curriculum map](curriculum-map.md), which is itself scoped under
the [curriculum catalogue](curriculum-catalogue.md). See decision [D-0009](decisions.md).

## Original-content policy (read first)

- **The public repository holds schema, code, contracts, docs, and tests only.** No question
  prompt, rubric structure, mark scheme, syllabus text, past-paper content, PDF, extracted text,
  diagram, or OCR output is committed here, and **no question or rubric data is seeded** by any
  migration. A test enforces this: `TestMigrations_SeedNoQuestionOrRubricContent` fails if any
  migration inserts into `questions`, `question_rubric_versions`, or `question_events`.
- Question prompts and rubric structures are **original** content, authored at runtime, in the
  database, by a future **private, approved editorial workflow**. That workflow is out of scope
  here — this task ships only the infrastructure it will write into.
- Original questions trace to syllabus/objective **IDs** (a curriculum-map node), never to copied
  source wording. A `sourceLocator` on the node is reference metadata, not source content.
- **No AI is involved.** This surface performs no AI generation, no Anthropic call, no OCR, no
  ingestion, and no question derivation. Marking and explanation generation are separate, future
  work; when they arrive they consume a **verified** question plus a **verified** rubric version —
  they do not author either.

## Prerequisite: the grounding chain

A question can only exist once every link below is in place. None of them is performed by this
task:

1. **Source rights approval** (T-0001) — a human approves the content source's rights metadata.
2. **Catalogue link confirmation** (T-0005) — a human links the source to a catalogue syllabus.
3. **Curriculum-map node authored and verified** (T-0006) — a reviewer/admin verifies a node
   grounded in that approved, linked source.
4. **Question authored** — only then can `POST /questions` pass the grounding gate.

The two seeded 0610/5090 content sources are still `pending` and unlinked, so today **no question
can be created for either syllabus.** That is the intended state.

## Schema

Tables live in `services/core/migrations` (0012–0014).

### `questions`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `syllabus_id` | UUID FK → `syllabuses` | immutable after creation |
| `curriculum_map_node_id` | UUID FK → `curriculum_map_nodes`, **NOT NULL** | must be **verified** and belong to `syllabus_id` |
| `response_type` | TEXT | `multiple_choice` \| `short_answer` \| `structured_response` |
| `language` | TEXT | opaque non-empty tag (e.g. `en`); no language registry exists yet |
| `prompt` | TEXT | **original** question body, written at runtime by the private workflow |
| `status` | TEXT | `draft` \| `verified` \| `retired` (default `draft`) |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### `question_rubric_versions`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `question_id` | UUID FK → `questions` | |
| `version` | INTEGER, `CHECK (version > 0)` | allocated server-side under a row lock on the question |
| `rubric` | JSONB | validation-safe structure (below); **original** editorial content |
| `max_marks` | INTEGER, `CHECK (max_marks > 0)` | must equal the sum of criterion marks |
| `status` | TEXT | `draft` \| `verified` (a version is superseded, never retired) |
| `created_by` / `reviewed_by` | TEXT | **verified Clerk subjects**; `reviewed_by` null while draft |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Uniqueness:** `UNIQUE (question_id, version)`.

**Immutability (database-enforced):** a `BEFORE UPDATE OR DELETE` trigger rejects every `DELETE`
and any `UPDATE` that changes `question_id`, `version`, `rubric`, `max_marks`, `created_by`, or
`created_at`. Only `status`, `reviewed_by`, and `updated_at` may change — i.e. verification, and
nothing else. Correcting a rubric therefore means appending a **new version**, which is what makes
a marked answer explainable by the exact rubric that produced it.

### `question_events` (immutable audit)

Append-only trail: `question_id`, `event_type` (`question_created`/`question_updated`/
`question_verified`/`question_retired`/`rubric_version_created`/`rubric_version_verified`),
`actor_id` (the **verified Clerk subject**), `event_time`, `changed_fields` (names only). A
`BEFORE UPDATE OR DELETE` trigger rejects any mutation, mirroring `content_source_events` /
`syllabus_events` / `curriculum_map_events`.

**The audit trail never stores a prompt, a rubric, a mark value, or any other field value** — only
field names such as `prompt`, `status`, `rubricVersion`.

## Rubric structure

The only accepted shape, validated in full before any write:

```json
{
  "criteria": [
    { "id": "c1", "marks": 2, "descriptor": "original editorial wording" },
    { "id": "c2", "marks": 1 }
  ]
}
```

- `criteria` must be a non-empty array (bounded at 200 entries).
- Each criterion needs a non-blank, unique `id` and a **positive integer** `marks`; `descriptor`
  is optional but must be non-blank when present.
- Unknown fields — on the document or on a criterion — are rejected.
- **Criterion marks must sum exactly to `maxMarks`**, so a rubric can never award more or fewer
  marks than the question is worth.

A rubric is validated twice: when the version is created, and again when it is verified (the point
at which it becomes usable). Failures return `400 invalid_rubric` or `400 invalid_max_marks` and
write nothing.

## The grounding gate

Every question write — create, `PATCH`, rubric-version create, rubric-version verify, question
verify, question retire — re-validates, inside the write transaction and **before any row, status
change, or audit event**:

1. the curriculum-map node exists → else `400 unknown_node`;
2. the node belongs to the question's syllabus → else `400 mismatched_node`;
3. the node is `verified` (draft and retired both fail) → else `400 unverified_node`;
4. the node's content source still passes the [T-0006 source gate](curriculum-map.md) — exists,
   `approved`, and linked to the same syllabus → else `400 unknown_source` /
   `unapproved_source` / `unlinked_source` / `mismatched_source`.

Step 4 is re-checked rather than trusted from node creation because a source can be un-approved,
unlinked, or re-linked after both the node and the question were written; step 3 likewise, because
a node can be retired.

**Operational consequence:** a question whose node or source has regressed is **frozen** — it
cannot be edited, given a rubric version, verified, *or retired* — until the node/source is
restored through the curriculum-map and content-source APIs. Freezing was chosen over letting a
lifecycle write proceed against grounding outside the rights gate, since that gate is the reason
this table exists. It matches the same trade-off documented for curriculum-map nodes.

**The gate is Core-only.** No other service — in particular the AI service — may create, update,
verify, or retire a question or rubric version, or evaluate the gate.

### Why `syllabusId` is immutable on a question

Changing it would silently invalidate the node link (and any rubric authored against it). Repoint
`curriculumMapNodeId` instead: that re-runs the whole gate, including the syllabus match, so an
attempted cross-syllabus move is rejected as `mismatched_node` rather than half-applied.

## Lifecycle

```
question:       draft --(verify)--> verified --(retire)--> retired
                draft --(retire)---------------------------^

rubric version: draft --(verify)--> verified   (superseded by a new version, never retired)
```

- **Create** always produces a `draft` question; `status` is never caller-settable.
- **PATCH** (`curriculumMapNodeId`, `responseType`, `language`, `prompt` — and nothing else) only
  succeeds while `status = draft`; otherwise `409 invalid_lifecycle_transition`. `status` itself is
  never PATCHable.
- **Rubric versions may only be appended to a `draft` question.** Version numbers are allocated
  server-side, are positive, and increase monotonically per question.
- **Verify a rubric version** (`draft → verified`) requires `question:verify` and a parent question
  that is `draft` or `verified`. Re-verifying a verified version is `409`.
- **Verify a question** (`draft → verified`) requires `question:verify` **and at least one verified
  rubric version** — otherwise `409 missing_verified_rubric`. A draft rubric version does not
  count.
- **Retire a question** (`draft`/`verified` → `retired`) requires `question:verify`. Retiring an
  already-retired question is `409`.
- **Reader endpoints return verified questions only**, so retiring a question removes it from every
  learner-facing read while keeping its history in `question_events`.

## API

All endpoints require a Clerk session bearer token (see [auth-setup.md](auth-setup.md)). The audit
actor, the rubric creator, and the rubric reviewer are always the verified Clerk subject — request
bodies carry no identity field.

### Strict request decoding

`POST` and `PATCH` accept exactly one JSON object and reject anything else with a stable `400
invalid_json` (the message never echoes the offending field back):

- **Unknown fields are rejected**, including identity fields (`actorId`, `reviewerId`), immutable
  fields (`syllabusId` on PATCH, `id`, `createdAt`, `updatedAt`), the lifecycle field (`status`),
  the server-allocated `version`, typos, and **case variants** of a real field (`Prompt`).
- **Trailing JSON values or junk after the object are rejected** (`{...}{}`, `{...}[1]`,
  `{...}not-json`).
- A rejected request is refused before any store call: no row mutation, no status transition, no
  audit event.

Both verbs enforce an explicit **case-sensitive allowlist** (`createQuestionFields`,
`updatablePatchFields`, `createRubricVersionFields` in `handlers.go`) in addition to a strict
struct decode. The allowlist is load-bearing twice over: Go's struct decoding matches field names
case-insensitively, so `DisallowUnknownFields` alone would accept `{"SyllabusId": ...}`, and it has
no effect at all on the `map[string]json.RawMessage` decode used to tell "field absent" from
"field present".

| Method & path | Permission | Roles | Notes |
| --- | --- | --- | --- |
| `GET /questions?syllabusId=...&curriculumMapNodeId=...` | `question:read` | editor, reviewer, admin | verified only; `syllabusId` required and must resolve to an **active** catalogue syllabus; node filter optional |
| `GET /questions/{id}` | `question:read` | editor, reviewer, admin | verified only (404 otherwise) |
| `POST /questions` | `question:create` | editor, reviewer, admin | creates a draft |
| `PATCH /questions/{id}` | `question:create` | editor, reviewer, admin | draft only |
| `POST /questions/{id}/rubric-versions` | `question:create` | editor, reviewer, admin | appends a draft version to a draft question |
| `GET /questions/{id}/rubric-versions` | `question_rubric:read` | editor, reviewer, admin | editorial read — exposes **draft** rubric structure |
| `POST /questions/{id}/rubric-versions/{version}/verify` | `question:verify` | reviewer, admin | draft → verified |
| `POST /questions/{id}/verify` | `question:verify` | reviewer, admin | draft → verified; needs a verified rubric |
| `POST /questions/{id}/retire` | `question:verify` | reviewer, admin | draft/verified → retired |

`401` missing/invalid token; `403` valid token lacking permission; `400` validation
(`missing_required_fields`, `blank_fields`, `invalid_response_type`, `invalid_json`,
`no_updatable_fields`, `no_changes`, `unknown_syllabus`, `unknown_node`, `unverified_node`,
`mismatched_node`, `unknown_source`, `unapproved_source`, `unlinked_source`, `mismatched_source`,
`invalid_rubric`, `invalid_max_marks`); `409` conflict (`invalid_lifecycle_transition`,
`missing_verified_rubric`, `duplicate_rubric_version`); `404` not found.

`500 internal_error` (database, scan, transaction, or other infrastructure failure) always returns
the same stable, generic message. Raw driver/Go error text is never forwarded to a client.

### Listing is validated, not silently empty

`GET /questions` resolves `syllabusId` against the catalogue **before** returning a result: an
unknown, malformed, or non-`active` syllabus is `400 unknown_syllabus`; a known active syllabus
with no verified questions is `200` with an empty `items` list. A typo is therefore never
indistinguishable from a real syllabus whose questions have not been authored yet — which matters
while no questions exist at all.

## Roles

Least privilege (see `services/core/internal/auth`). Learner and unknown roles have **no** question
or rubric access at all.

| Role | read verified questions | create/update draft question | create draft rubric version | list rubric versions | verify rubric / verify question / retire |
| --- | --- | --- | --- | --- | --- |
| learner | — | — | — | — | — |
| editor | ✓ | ✓ | ✓ | ✓ | — |
| reviewer | ✓ | ✓ | ✓ | ✓ | ✓ |
| admin | ✓ | ✓ | ✓ | ✓ | ✓ |

Rubric listing has its own permission (`question_rubric:read`) rather than reusing
`question:read`, because it exposes draft rubric structure — not just verified questions — and
that separation lets a future read-only or learner-facing question surface be added without
handing out the marking scheme.

## Audit

Every successful mutation appends one `question_events` row inside the same transaction as the
write, so a failed audit insert rolls back the mutation. Each row records the **verified Clerk
subject** and the **names** of the fields involved:

| Action | `event_type` | `changed_fields` |
| --- | --- | --- |
| create question | `question_created` | `syllabusId`, `curriculumMapNodeId`, `responseType`, `language`, `prompt` |
| PATCH question | `question_updated` | only the names that actually changed |
| verify / retire question | `question_verified` / `question_retired` | `status` |
| create rubric version | `rubric_version_created` | `rubricVersion` |
| verify rubric version | `rubric_version_verified` | `rubricVersionStatus` |

## Future AI boundary

Nothing in this package calls an AI model, and nothing may. When marking and explanation
generation are built (per D-0003: Anthropic only, Haiku for routine work, Sonnet for complex
marking), the boundary is:

- **AI consumes, never authors.** A marking or explanation call reads a **verified** question and a
  **verified** rubric version. It may not create, update, verify, or retire either, and it may not
  evaluate or bypass the grounding gate — Core remains the sole authority.
- **The rubric version is part of the cache key.** The canonical explanation cache key is
  `question + syllabus + rubric + language + explanation version`; the immutable rubric **version**
  is what makes the `rubric` component well-defined and a cached explanation reproducible.
- **No ingestion path is opened.** Questions are original and human-authored; nothing here derives
  a question from a source, and the T-0001 ingestion gate still blocks every non-approved source.
