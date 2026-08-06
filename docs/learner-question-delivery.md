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

## Update (T-0015 review)

Two review findings were fixed on top of the original implementation (still task-status
`review`, not released):

1. **Canonical-rubric ownership gate.** The eligibility query joined
   `question_rubric_versions rv ON rv.id = q.canonical_rubric_version_id` without also requiring
   `rv.question_id = q.id`. `canonical_rubric_version_id` is a plain foreign key with no
   database-level constraint tying it to the owning question, so — while no editorial write path
   can currently produce this state — the read-time gate itself did not enforce ownership. Fixed
   by adding `AND rv.question_id = q.id` to the join. Covered by a new integration regression
   (`TestPostgresStore_Integration_CanonicalRubric_OwnershipGate`): two otherwise-eligible
   questions are seeded, question A's `canonical_rubric_version_id` is pointed at question B's
   verified, current-revision rubric via direct disposable-test setup, and the test asserts A is
   excluded from both `GetQuestion` and `ListQuestions` while B remains eligible and unaffected.
   No canonical-rubric fallback exists anywhere in this package (confirmed by inspection — the
   query has exactly one JOIN to `question_rubric_versions`, no `OR`/`COALESCE`/latest-version
   path).
2. **Learner-safe active-syllabus discovery.** The practice screen previously asked a learner to
   type an opaque syllabus UUID into a text field labelled "syllabus picker." Added `GET
   /learner/syllabuses` (same `learner_question:read` permission, same package, no new
   permission) returning the explicit `Syllabus` projection — `id`, `board`, `syllabusCode`,
   `qualification`, `track`, `displayName` — for every catalogue syllabus currently `active`,
   read directly from the `syllabuses` table (this package still does not import `catalogue`, for
   the same reason it does not import `question`). The web BFF gained a matching closed
   `listLearnerSyllabuses` operation (GET, no params) in the same `LearnerOperation` union, and
   `/dashboard/practice` now fetches this list on mount and renders an accessible `<select>`
   (loading/empty/error-with-retry states) instead of a text input. Eligible-question fetching is
   still fully separate: it never runs until the learner explicitly selects a syllabus from the
   dropdown and submits. The optional curriculum-map node filter remains a plain text field,
   explicitly labelled "temporary, developer-only" — no learner-facing curriculum-map browse
   endpoint was added, matching D-0017's original scope decision.

## Known exclusions (deliberately out of scope)

- No learner-facing curriculum-catalogue or curriculum-map browse endpoint. The practice
  screen's syllabus/node inputs are plain validated ID fields, not a picker backed by a new
  read permission — avoids widening `content_catalogue:read`/`curriculum_map:read` to the
  learner role, which was not requested.
- No Exam Mode, attempt, session, marking, explanation, or AI integration.
- No seeded or sample question content anywhere in this repository or its tests (D-0005); all
  test fixtures use opaque generated strings.
