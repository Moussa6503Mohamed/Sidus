# Canonical rubric selection

T-0014 makes rubric choice explicit. A reviewer/admin must select exactly one verified rubric
version when verifying a draft question. Core never chooses latest, highest-numbered, or any other
rubric automatically.

## Invariant

Every question verified through current API has non-null `canonical_rubric_version_id` pointing
to a rubric row that, at selection time:

- belongs to that question;
- has `status = verified`;
- has `question_revision = questions.content_revision`.

Question and selected rubric rows are locked in one transaction. Grounding/source approval is
rechecked before selection. Verification writes `status = verified`, canonical rubric id, and
immutable audit event atomically. Failure writes none.

No replacement occurs automatically or through current API. Verified question content remains
immutable. Selection stays stable for reproducible future delivery and marking.

## Migration and historical data

Migration 0018 adds nullable `questions.canonical_rubric_version_id`, FK to
`question_rubric_versions(id)`, plus partial index for non-null references. Migration is additive
and rerunnable. It contains no update, insert, seed, or selection query.

Draft questions and historical verified questions remain null. No latest-rubric backfill occurs.
Reviewer/admin may repair historical verified null through:

```http
POST /questions/{id}/canonical-rubric
Content-Type: application/json

{ "rubricVersion": 3 }
```

Repair applies same owned/verified/current and grounding checks, changes only canonical field,
and rejects draft, retired, or already-selected questions. Editors, learners, and unknown roles
cannot use endpoint.

## Verification request

`POST /questions/{id}/verify` now requires exact body:

```json
{ "rubricVersion": 3 }
```

`rubricVersion` is positive per-question version number. Unknown fields, case variants,
body-supplied identity, duplicate keys, null, non-integers, and trailing JSON are rejected before
store mutation. Foreign/unknown, draft, and stale selections return stable domain errors. Database
or internal text is never returned.

## Audit and read boundary

Question verification records changed field names `status` and `canonicalRubricVersionId`.
Historical repair records `canonical_rubric_selected` with only `canonicalRubricVersionId`. Actor
always comes from verified Clerk subject. Events never record version values, prompts, options,
answer keys, rubric text, or request-body identity.

Editorial question reads include nullable `canonicalRubricVersionId`; rubric UI shows marker. No
learner route exists. Future learner projection must omit canonical rubric id, rubric document,
and `answerKey`.

## BFF and UI

Editorial BFF adds one fixed allowlisted repair operation. Selection bodies receive JSON envelope
and exact positive-version validation before fixed Core call; Core remains strict shape and
authorization authority. Existing missing-config/token behavior, redirect refusal, generic
upstream failures, and Core 5xx sanitization remain shared.

Reviewer/admin UI lists only verified rubric versions matching current question revision for
selection. Verification stays disabled until explicit selection and confirmation. Historical
verified null rows expose repair selection and confirmation. Editors see marker but no selection
or replacement controls.
