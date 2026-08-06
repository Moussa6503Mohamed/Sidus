# Learner question delivery

T-0015 adds the first learner-facing surface over the private question/rubric model built in
T-0007/T-0013/T-0014. It is strictly read-only: no attempts, sessions, marking, timers,
explanations, or Exam Mode. See D-0017.

## Projection

`GET /learner/questions` and `GET /learner/questions/{id}` return only the explicit,
exhaustive `LearnerQuestion` contract (`packages/shared/src/contracts.ts`, mirrored
independently — no shared Go type — by `services/core/internal/learner.Projection`):

```json
{
  "id": "…",
  "syllabusId": "…",
  "curriculumMapNodeId": "…",
  "responseType": "multiple_choice",
  "language": "en",
  "prompt": "…",
  "options": [{ "id": "…", "label": "…" }],
  "contentRevision": 1
}
```

`options` is `null` for `short_answer`/`structured_response`.

### Excluded — never present at any layer

`status`, `canonicalRubricVersionId`, any rubric structure, `answerKey`, marks, event/audit
data, actor/reviewer identity, `createdAt`/`updatedAt`, and internal source metadata (source
id, URL, hash, licence). The `learner` package does not import the `question` package's types,
so a future edit to the editorial `Question`/`QuestionRubricVersion` shape cannot widen this
projection by accident — every learner-safe field is listed by hand in both the Go and
TypeScript definitions.

## Eligibility gate

A question is returned only while **every** condition holds, re-checked on every read (a single
SQL query joining `questions`, `curriculum_map_nodes`, `content_sources`, and
`question_rubric_versions` — never cached, never trusted from a prior write):

1. `questions.status = 'verified'`.
2. `questions.canonical_rubric_version_id` is set (D-0016's explicit reviewer selection — Core
   never falls back to "latest rubric").
3. The canonical rubric version is itself `verified` **and** its `question_revision` equals the
   question's current `content_revision` (D-0009's staleness rule flows straight through: an
   edited question loses learner visibility until a current rubric is verified and re-selected).
4. The grounding curriculum-map node is `verified`.
5. The node's content source is `approved` and its `catalogue_syllabus_id` still equals the
   question's `syllabus_id` (the T-0006 source gate, re-checked here exactly as it is on every
   editorial write).

A question that exists but fails any gate is reported identically to one that does not exist:
`404 not_found` from `GET /learner/questions/{id}`, silently absent from `GET
/learner/questions`. Distinguishing "exists but ineligible" from "does not exist" would leak
draft/retired/ungrounded question existence to a role that must never see it.

## Access

New permission `learner_question:read` is held by **every** recognized role — `learner`,
`editor`, `reviewer`, `admin` — the first Core permission a `learner`-role session may use.
Unknown/missing role is denied by `auth.Protect`, same as every other Core route.

## Threat boundary

- **Core**: dedicated `services/core/internal/learner` package and routes. The editorial
  `/questions*` routes, `question:read` permission, and internal `Question`/`RubricVersion`
  types are untouched — this is not a filtered view of the same handler, it is a separate
  package with its own SQL and its own response type.
- **Web BFF**: `apps/web/lib/learner/core-proxy.ts` defines its own closed `LearnerOperation`
  union (`listLearnerQuestions`, `getLearnerQuestion`) and `callCoreLearner`, deliberately
  separate from `EditorialOperation`/`callCore` — a change to one union cannot widen the other.
  Same fail-closed contract as the T-0009 editorial BFF: missing `SIDUS_CORE_API_URL` → `503`
  before touching auth; ids/query values validated before any Clerk lookup; missing/invalid
  session token → `401` before any network call; Core redirects refused (`redirect: "error"`);
  every Core `5xx` body is discarded and replaced with a generic `502`; nothing (token, body,
  Core URL, upstream response body) is ever logged. GET-only — there is no write path.
- **Web UI**: `/dashboard/practice` renders only the safe projection. Selecting an MCQ option
  sets local highlight state only — there is no submit, mark, answer reveal, timer, attempt/
  session creation, or AI call anywhere in the workspace or its data path.

## Known exclusions (deliberately out of scope)

- No learner-facing curriculum-catalogue or curriculum-map browse endpoint. The practice
  screen's syllabus/node inputs are plain validated ID fields, not a picker backed by a new
  read permission — avoids widening `content_catalogue:read`/`curriculum_map:read` to the
  learner role, which was not requested.
- No Exam Mode, attempt, session, marking, explanation, or AI integration.
- No seeded or sample question content anywhere in this repository or its tests (D-0005); all
  test fixtures use opaque generated strings.
