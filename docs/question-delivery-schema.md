# Question delivery schema

T-0013 adds deterministic multiple-choice authoring data to existing private question/rubric
model. It does **not** add learner delivery, attempts, sessions, marking, explanations, timers, or
Exam Mode. See D-0015 and [question-rubric-model.md](question-rubric-model.md).

## Ownership and shape

Question-owned `options` is nullable JSONB. Existing rows remain `NULL`. New Core writes enforce:

```json
[
  { "id": "stable-editorial-id", "label": "original editorial label" }
]
```

- `options` is required for `multiple_choice` and absent for `short_answer` and
  `structured_response`.
- There are 2–6 ordered options. Reordering changes display order.
- Each exact object has only `id` and `label`. IDs are non-blank, unique, and at most 64 Unicode
  code points. Labels are non-blank and at most 1,000 Unicode code points.
- IDs are stable references. Labels remain original question content, never copied source prose.
- Any options change is one normal question content update: `content_revision` increments exactly
  once and older rubric versions become stale. Audit stores only `options` and `contentRevision`
  field names, never IDs, labels, or values.
- Switching an MCQ to a non-MCQ atomically clears options. Switching to MCQ requires valid options
  in the same PATCH.

Rubric-owned `answerKey` is optional in shared shape but response-type constrained by Core:

```json
{
  "criteria": [{ "id": "criterion-id", "marks": 1 }],
  "answerKey": { "correctOptionId": "stable-editorial-id" }
}
```

- MCQ rubric creation requires exact `answerKey.correctOptionId`, matched under question row lock
  against one current option ID.
- Non-MCQ rubrics reject `answerKey`; they remain criteria-only. No exact-match short/structured
  answer format exists.
- Existing rubric row immutability covers `answerKey` because it is inside immutable `rubric`
  JSONB. A label or option change stales, never rewrites, an older answer key.
- Rubric-version verify keeps existing structural revalidation. Stale versions remain verifiable
  and auditable, but cannot unblock question verification because revision must be current.

## Strict input boundary

Question POST/PATCH and rubric-version POST accept exactly one JSON object. Exact token decoding
rejects unknown or case-variant fields, duplicate keys, invalid types, and trailing JSON before any
store call. Option and answer-key nested objects use same exact rules. Existing criteria and
maximum-mark rules remain unchanged.

## Editorial workflow

Existing fixed-operation BFF routes forward question/rubric bodies unchanged; no new proxy target
or learner surface is added. Draft MCQ editor permits add/remove/reorder and ID/label edits with no
default content. Rubric editor renders correct-option selector only for MCQ and populates it only
from current question options. Verified/retired question content stays locked. Revision and rubric
freshness remain visible.

## Answer-key safety boundary

Current editorial reads include rubric `answerKey`, as reviewers need it. No learner endpoint exists.
Any future learner contract must use a dedicated response projection that omits rubric documents and
`answerKey`; reusing editorial `QuestionRubricVersion` would violate this boundary.

T-0014 also adds editorial-only nullable `canonicalRubricVersionId`. Future learner projection must
omit this id as well as rubric documents and `answerKey`. Core will use explicit canonical relation
internally for reproducible future delivery/marking; no learner route is added here.
