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

The seeded 0610 source remains `pending` and unlinked, and no 9700 source exists, so today **no
question can be created for either active Biology syllabus.** Historical 5090 is retired. That is
the intended state.

## Schema

Tables live in `services/core/migrations` (0012–0014); migration 0015 adds the content-revision
columns described below.

### `questions`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `syllabus_id` | UUID FK → `syllabuses` | immutable after creation |
| `curriculum_map_node_id` | UUID FK → `curriculum_map_nodes`, **NOT NULL** | must be **verified** and belong to `syllabus_id` |
| `response_type` | TEXT | `multiple_choice` \| `short_answer` \| `structured_response` |
| `language` | TEXT | opaque non-empty tag (e.g. `en`); no language registry exists yet |
| `prompt` | TEXT | **original** question body, written at runtime by the private workflow |
| `options` | JSONB, nullable | ordered original `{id,label}` options; Core requires them only for new `multiple_choice` writes |
| `status` | TEXT | `draft` \| `verified` \| `retired` (default `draft`) |
| `content_revision` | INTEGER, `CHECK (> 0)` | starts at 1; **+1 per successful content update** |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

### `question_rubric_versions`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `question_id` | UUID FK → `questions` | |
| `version` | INTEGER, `CHECK (version > 0)` | allocated server-side under a row lock on the question |
| `question_revision` | INTEGER, `CHECK (> 0)` | the question's `content_revision` at creation — the content this rubric was reviewed against |
| `rubric` | JSONB | validation-safe structure (below); **original** editorial content |
| `max_marks` | INTEGER, `CHECK (max_marks > 0)` | must equal the sum of criterion marks |
| `status` | TEXT | `draft` \| `verified` (a version is superseded, never retired) |
| `created_by` / `reviewed_by` | TEXT | **verified Clerk subjects**; `reviewed_by` null while draft |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Uniqueness:** `UNIQUE (question_id, version)`.

**Immutability (database-enforced):** a `BEFORE UPDATE OR DELETE` trigger rejects every `DELETE`
and any `UPDATE` that changes `question_id`, `version`, `question_revision`, `rubric`, `max_marks`,
`created_by`, or `created_at`. Only `status`, `reviewed_by`, and `updated_at` may change — i.e.
verification, and nothing else. Correcting a rubric therefore means appending a **new version**,
which is what makes a marked answer explainable by the exact rubric that produced it.
`question_revision` is inside the immutable set on purpose: question verification reads it, so a
direct SQL rewrite would otherwise be a way to re-point a stale version at current content.

## Content revision: a rubric belongs to the question content it was reviewed against

A rubric is only meaningful for the question wording, response type, language, and objective it was
written for. Without a link between the two, a draft question could gain a **verified** rubric and
then be edited, and question verification would still accept a rubric nobody reviewed against the
current text.

- `questions.content_revision` starts at **1** and is incremented by **exactly one**, inside the
  transaction of every **successful** draft content update (`curriculumMapNodeId`, `responseType`,
  `language`, `prompt`, `options`). `no_changes`, a validation failure, a grounding failure, a strict-decoding
  rejection, and a lifecycle transition (verify/retire) all leave it untouched. It is never
  caller-settable.
- Every rubric version stores the `content_revision` that was current when it was created, under
  the same row lock that allocated its version number.
- A rubric version is **current** only while its `question_revision` equals the question's
  `content_revision`.
- **Verifying a question requires at least one verified rubric version for the current revision.**
  A verified version from an older revision does not count.

An edit therefore **stales** older versions rather than deleting or downgrading them: they stay
`verified`, immutable, and readable to editorial roles, and they remain the correct record of what
was marked at that revision. The remedy is to append a new version — which picks up the current
revision automatically — and have a reviewer verify it; the question can then be verified.

Because the two failures need different fixes, they have different stable codes:

| Situation | Response |
| --- | --- |
| No verified rubric version at all | `409 missing_verified_rubric` |
| Verified versions exist, all from older revisions | `409 missing_current_verified_rubric` |

### `question_events` (immutable audit)

Append-only trail: `question_id`, `event_type` (`question_created`/`question_updated`/
`question_verified`/`question_retired`/`rubric_version_created`/`rubric_version_verified`),
`actor_id` (the **verified Clerk subject**), `event_time`, `changed_fields` (names only). A
`BEFORE UPDATE OR DELETE` trigger rejects any mutation, mirroring `content_source_events` /
`syllabus_events` / `curriculum_map_events`.

**The audit trail never stores a prompt, a rubric, a mark value, or any other field value** — only
  field names such as `prompt`, `options`, `contentRevision`, `status`, `rubricVersion`.

## Multiple-choice options

`options` is required on new `multiple_choice` writes and prohibited on `short_answer` and
`structured_response`. It is an ordered array of 2–6 exact `{ "id", "label" }` objects. IDs are
non-blank, unique, and at most 64 Unicode code points; labels are non-blank original editorial text
and at most 1,000 code points. Unknown keys, case variants, duplicates, wrong types, and trailing
JSON are rejected. Existing pre-T-0013 rows remain valid with `NULL` options until edited. Changing
options, including order/ID/label, is one content update. Switching MCQ to non-MCQ clears options;
switching to MCQ requires valid options in same write. See [question-delivery-schema.md](question-delivery-schema.md).

## Rubric structure

The only accepted shape, validated in full before any write:

```json
{
  "criteria": [
    { "id": "c1", "marks": 2, "descriptor": "original editorial wording" },
    { "id": "c2", "marks": 1 }
  ],
  "answerKey": { "correctOptionId": "stable-option-id" }
}
```

- **Every key is matched exactly and case-sensitively.** The document accepts `criteria` and optional
  `answerKey`; a criterion accepts `id`, `marks`, and `descriptor`, while answer key accepts only
  `correctOptionId`. `Criteria`,
  `ID`, `Marks`, `Descriptor`, a key with stray whitespace, an unknown key, a key at the wrong
  level, and a **duplicate** key are all rejected.
- `criteria` must be a non-empty array (bounded at 200 entries) of **objects**.
- Each criterion needs a non-blank, unique `id` (a JSON string) and a **positive integer** `marks`
  (a JSON number, not `"3"`, not `1.5`); `descriptor` is optional, may be explicitly `null`, and
  must be a non-blank string otherwise.
- Trailing JSON after the document is rejected.
- **Criterion marks must sum exactly to `maxMarks`**, so a rubric can never award more or fewer
  marks than the question is worth.
- MCQ creation requires `answerKey.correctOptionId` to equal one current stable option ID under
  question row lock. Non-MCQ creation rejects `answerKey`; those response types remain criteria-only.
- Answer key is immutable with rest of rubric JSON. It is visible only through current editorial
  rubric reads; no learner route exists, and future learner projections must omit it.

Validation is written against `encoding/json`'s token API rather than a struct or map decode,
because neither of those can enforce the schema: Go matches struct field names **case-insensitively**
(so `DisallowUnknownFields` still accepts `{"Criteria":[{"ID":…}]}`), and both a struct and a map
silently keep the last value of a duplicated key instead of rejecting it. This mirrors the
case-sensitive allowlist the HTTP handlers already apply to request bodies.

A rubric is structurally validated when created and verified. Response-type/option matching occurs
at creation under question lock; current-revision verification later guarantees usable rubric was
created against same question content. Failures return `400 invalid_rubric` or `400 invalid_max_marks`
and write nothing.

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
- **PATCH** (`curriculumMapNodeId`, `responseType`, `language`, `prompt`, `options` — and nothing else) only
  succeeds while `status = draft`; otherwise `409 invalid_lifecycle_transition`. `status` itself is
  never PATCHable. A successful PATCH increments `content_revision` by one and so stales every
  existing rubric version for verification purposes.
- **Rubric versions may only be appended to a `draft` question.** Version numbers are allocated
  server-side, are positive, and increase monotonically per question; each version is stamped with
  the question's current `content_revision`.
- **Verify a rubric version** (`draft → verified`) requires `question:verify` and a parent question
  that is `draft` or `verified`. Re-verifying a verified version is `409`. A version from an older
  revision may still be verified — it simply does not unblock question verification.
- **Verify a question** (`draft → verified`) requires `question:verify` **and at least one verified
  rubric version for the question's current revision** — otherwise `409 missing_verified_rubric`
  (nothing verified) or `409 missing_current_verified_rubric` (only stale versions verified). A
  draft rubric version does not count.
- **Retire a question** (`draft`/`verified` → `retired`) requires `question:verify`. Retiring an
  already-retired question is `409`.
- **Editorial reader endpoints return every lifecycle state** to `question:read` holders so staff
  can discover and reopen drafts and audit retired records. `GET /questions` accepts
  `status=draft|verified|retired|all`; omission and `all` both mean every status. No learner has
  `question:read`, and no learner-facing question route exists.

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
- **Duplicate keys are rejected**, at request, option, rubric, criterion, and answer-key levels.
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
| `GET /questions?syllabusId=...&curriculumMapNodeId=...&status=...` | `question:read` | editor, reviewer, admin | every status by default; optional `draft`/`verified`/`retired`/`all`; `syllabusId` required and active; node filter validated |
| `GET /questions/{id}` | `question:read` | editor, reviewer, admin | any lifecycle status |
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
`invalid_options`,
`invalid_rubric`, `invalid_max_marks`, `invalid_status`); `409` conflict (`invalid_lifecycle_transition`,
`missing_verified_rubric`, `missing_current_verified_rubric`, `duplicate_rubric_version`); `404`
not found.

`500 internal_error` (database, scan, transaction, or other infrastructure failure) always returns
the same stable, generic message. Raw driver/Go error text is never forwarded to a client.

### Listing is validated, not silently empty

`GET /questions` resolves **both** filters against the database before returning a result, so a
typo is never indistinguishable from a real filter whose questions have not been authored yet —
which matters while no questions exist at all:

1. `syllabusId` — unknown, malformed, or non-`active` → `400 unknown_syllabus` (checked first, so
   a bad syllabus wins over the node filter);
2. `curriculumMapNodeId`, when supplied — unknown or malformed → `400 unknown_node`; belonging to
   another syllabus → `400 mismatched_node`.

A known active syllabus, plus a node of that syllabus if supplied, with no matching questions is
`200` with an empty `items` list.

The node **filter** is checked more weakly than the [grounding gate](#the-grounding-gate): it
requires only that the node exists and belongs to the syllabus, not that it is verified or that its
source still passes the gate. A reader may legitimately filter by a node that has since been
retired, and gets an empty list rather than an error.

## Roles

Least privilege (see `services/core/internal/auth`). Learner and unknown roles have **no** question
or rubric access at all.

| Role | read editorial questions | create/update draft question | create draft rubric version | list rubric versions | verify rubric / verify question / retire |
| --- | --- | --- | --- | --- | --- |
| learner | — | — | — | — | — |
| editor | ✓ | ✓ | ✓ | ✓ | — |
| reviewer | ✓ | ✓ | ✓ | ✓ | ✓ |
| admin | ✓ | ✓ | ✓ | ✓ | ✓ |

Rubric listing has its own permission (`question_rubric:read`) rather than reusing
`question:read`, because rubric structure is a distinct sensitive surface. That separation lets a
future public verified-question surface omit marking criteria without changing editorial access.

## Audit

Every successful mutation appends one `question_events` row inside the same transaction as the
write, so a failed audit insert rolls back the mutation. Each row records the **verified Clerk
subject** and the **names** of the fields involved:

| Action | `event_type` | `changed_fields` |
| --- | --- | --- |
| create question | `question_created` | `syllabusId`, `curriculumMapNodeId`, `responseType`, `language`, `prompt` |
| PATCH question | `question_updated` | only the names that actually changed, plus `contentRevision` |
| verify / retire question | `question_verified` / `question_retired` | `status` |
| create rubric version | `rubric_version_created` | `rubricVersion`, `questionRevision` |
| verify rubric version | `rubric_version_verified` | `rubricVersionStatus` |

## Future AI boundary

Nothing in this package calls an AI model, and nothing may. When marking and explanation
generation are built (per D-0003: Anthropic only, Haiku for routine work, Sonnet for complex
marking), the boundary is:

- **AI consumes, never authors.** A marking or explanation call reads a **verified** question and a
  **verified** rubric version for that question's current `contentRevision` — the pairing the
  revision stamp exists to guarantee. It may not create, update, verify, or retire either, and it may not
  evaluate or bypass the grounding gate — Core remains the sole authority.
- **The rubric version is part of the cache key.** The canonical explanation cache key is
  `question + syllabus + rubric + language + explanation version`; the immutable rubric **version**
  is what makes the `rubric` component well-defined and a cached explanation reproducible.
- **No ingestion path is opened.** Questions are original and human-authored; nothing here derives
  a question from a source, and the T-0001 ingestion gate still blocks every non-approved source.
