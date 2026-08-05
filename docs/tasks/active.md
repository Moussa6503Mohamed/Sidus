# Active tasks

## T-0013 — Deterministic MCQ delivery schema

- **Status:** review
- **Owner:** Codex
- **Type:** migration, code+tests, UI/design, documentation
- **Context:** T-0007 provides original questions, content revisions, immutable rubric versions,
  grounding, and audit rules. T-0012 provides private editorial question/rubric UI and BFF.
- **Goal:** Add safe question-owned multiple-choice options and rubric-versioned canonical correct
  option support without adding learner delivery or marking behavior.
- **Scope:** Add nullable `questions.options` JSONB through an additive rerunnable migration; shared
  option/answer-key contracts; strict Core validation and persistence; editorial BFF/UI editing;
  regression, migration, API, and UI tests; architecture docs and handoff; track read-only gap audit.
- **Allowed files:** `services/core/migrations/0017*`; question package and tests under
  `services/core/internal/question`; migration/integration tests needed for 0017; relevant shared
  contracts/tests under `packages/shared`; T-0012 question editorial BFF/UI/tests under `apps/web`;
  `docs/tasks/active.md`, `docs/reviews/T-0013-assessment-delivery-gap-audit.md`,
  `docs/question-delivery-schema.md`, `docs/question-rubric-model.md`, `docs/decisions.md`,
  `docs/local-setup.md`, `docs/handoffs/T-0013.md`, and `CLAUDE.md`.
- **Forbidden files:** migrations `0001`–`0016`; `.claude/`, `.claude-flow/`, `.env.local`, PDFs,
  ZIPs, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx`, design bundles, source/content data, unrelated files.
- **Non-goals:** Learner routes/BFF/UI, sessions, attempts, AI/Anthropic, marking, explanations,
  timers, Exam Mode, Redis, OCR/ingestion, seeded or hard-coded question/rubric/source/node prose.
- **Plan-first answers:** Existing rows have no options and must remain valid; post-state adds
  nullable JSONB only; clean bootstrap and initialized databases both run 0017; rerun is safe;
  down path removes only 0017 column on disposable test data; runtime migration and read/write
  paths receive real integration coverage.
- **Acceptance criteria:** Exact contract and JSON key validation; MCQ requires 2–6 unique bounded
  options and rubric `answerKey.correctOptionId` matching current options; non-MCQ rejects both;
  option PATCH increments content revision once and stales old rubrics; rejected/no-op writes do
  not mutate; events contain names only; editorial draft-MCQ option editing and current-option key
  selection contain no sample content; all existing rules remain intact.
- **Validation commands:** web Vitest/typecheck/build; shared strict TypeScript; Go gofmt/build/vet/
  unit; disposable `sidus-test` migrate/rerun/integration/down; Python pytest; Compose config checks;
  `git diff --check`.
- **Security/privacy:** Core remains authorization and validation authority; BFF stays fixed-route,
  token-server-side, redirect-refusing, fail-closed, and 5xx-sanitizing; no content or answer key
  enters audit values; no learner response is added.
- **Review checklist:** Migration additive/rerunnable; validation happens before mutation; answer
  key bound to current option IDs; rubric immutability preserved; role/grounding/source/lifecycle
  gates unchanged; no protected/content files staged; full validation recorded.
- **Handoff requirements:** `docs/handoffs/T-0013.md` records files, decisions, exact commands/results,
  risks/deferred learner-delivery safety, audit tracking, commit hash or pre-commit state.
- **Stop condition:** Leave status `review` after implementation, validation, docs, one scoped commit;
  never mark `done`, push, or touch protected files. Independent review required for `done`.
