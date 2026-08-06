# Practice Mode MCQ marking

T-0016 adds single-question, one-shot Practice Mode MCQ attempts. It does not add Exam Mode.

## Verified feedback schema

For `multiple_choice`, every newly created or verified rubric must include exactly:

```json
{
  "criteria": [],
  "answerKey": { "correctOptionId": "<current-option-id>" },
  "feedback": {
    "correctExplanation": "<original-editorial-text>",
    "incorrectExplanations": [
      { "optionId": "<current-incorrect-option-id>", "explanation": "<original-editorial-text>" }
    ]
  }
}
```

`criteria` retains existing non-empty marks rules; abbreviated values above document shape only,
not valid content or seed data. Core requires non-blank original editorial explanation text,
exactly one wrong explanation for every current option except correct option, and exact
case-sensitive keys. Duplicate/missing/foreign/stale/correct-option rationale IDs, unknown keys,
duplicate JSON keys, invalid types, and trailing JSON are rejected. Non-MCQ rubrics reject
`answerKey` and `feedback`. Historical rows receive no backfill and remain immutable history;
old MCQ rows without verified feedback cannot create Practice attempts.

Feedback must be authored and verified against current question revision. It must never come from
a PDF, past paper, mark scheme, textbook, source extract, LLM, generated text, cached explanation,
or a fallback/latest rubric.

## Attempt lifecycle

Migration 0019 adds `learner_attempts` and append-only `learner_attempt_events`. Creation requires
verified Clerk subject plus recognized role. Core rechecks every T-0015 gate in one transaction,
requires MCQ feedback completeness, then pins question content revision, explicit canonical rubric
version ID, and maximum marks. Pins never change.

`POST /learner/questions/{id}/attempts` returns only `attemptId`, `questionId`, `status`, and
`maxMarks`. `POST /learner/attempts/{id}/submit` accepts exactly
`{"selectedOptionId":"..."}`. Owner-only row locking makes `open -> submitted` one-way and
concurrent/replayed submission a stable `409 attempt_already_submitted`. Option membership comes
from pinned immutable rubric coverage, so later question/source lifecycle changes do not re-mark or
invalidate existing attempt records. Correct selection earns `maxMarks`; wrong selection earns 0.

Successful submit returns exactly attempt/question IDs, selected/correct option IDs, correctness,
awarded/max marks, and pinned canonical feedback. It never returns raw rubric/criteria, source or
actor metadata, events/timestamps, pinned revision/canonical ID, or internal errors.

## Web boundary and UI

Learner BFF remains separate from editorial BFF. Closed operations map only fixed learner routes.
IDs and exact submit body are validated before Clerk/Core access; missing config/token fail closed,
redirects fail, and Core 5xx bodies become generic 502 responses.

`/dashboard/practice` allows explicit option selection and submit, blocks duplicate submit, labels
selected and correct choices with text plus styling, and renders score, correct explanation, and
every wrong-option explanation. Loading, empty, retry, error, and access-denied states remain.
No timer, auto-submit, pre-submit reveal, paper/session flow, progress score, AI, or Exam Mode
control exists.

## Review hardening

Both learner write BFF routes read request bodies through one 4096-byte hard cap matching Core.
An oversized valid `Content-Length` fails before body reading; missing, malformed, or inaccurate
length still goes through byte-counted streaming and can never cause unbounded buffering. Existing
empty-create and exact-submit contracts remain unchanged and finish validation before Clerk/Core.

Attempt creation also validates persisted marking data as exact sets. Current option IDs must be
unique and contain the correct ID exactly once. Incorrect explanation IDs must be unique and equal
every current non-correct option ID, with no foreign or correct-option entry. Corrupt persisted
rows return the same safe not-found response as ineligible questions and create no attempt/event.
