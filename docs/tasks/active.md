# Active tasks

## T-0029 — Learner answer-type expansion

**Status:** review
**Type:** Core schema/API + learner BFF/UI + editorial authoring + tests.
**Owner:** Codex.

### Goal

Deliver real learner answer capture beyond MCQ: deterministic multi-select, exact short answer,
and numeric answers; let written short/structured/essay responses submit safely for later review.

### Scope

- Add response-aware question configuration, answer-key validation, learner-safe projections, and
  attempt answers without leaking marking keys before submission.
- Add deterministic marking for multi-select, normalized exact short answers, and numeric answers
  with an explicit configured tolerance.
- Let short/structured/essay responses be captured as text and return `pending_review`; never
  pretend they are automatically marked.
- Update editorial authoring and learner Practice/Exam renderers for supported response types.
- Preserve existing MCQ behavior and historical attempts.

### Allowed files

- `services/core/migrations/**`
- `services/core/internal/{question,learner,auth}/**`
- `services/core/main.go`
- `packages/shared/src/contracts.ts`
- `apps/web/lib/{editorial,learner}/**`
- `apps/web/app/api/{editorial,learner}/**`
- `apps/web/app/dashboard/{editorial/questions,practice,exam}/**`
- `apps/web/e2e/**`
- `docs/{tasks/active.md,handoffs/T-0029.md,question-delivery-schema.md,practice-mcq-marking.md,decisions.md}`
- `CLAUDE.md`

### Boundaries

- No private source folders, PDFs, source text, question content, secrets, direct database edits,
  source/provenance disclosure, AI marking, generated questions, diagrams, or external assets.
- Existing structured-response records remain compatible. Human review/marking UI, rich data
  stimuli, graphs, diagrams, matching, and file attachments are deferred follow-on work.

### Acceptance criteria

- Learners can answer MCQ, multi-select, exact short answer, numeric, and written response
  questions in Practice and Exam.
- MCQ/multi-select/exact/numeric results mark deterministically after submission only.
- Written responses persist safely and show pending review, with no false score/correctness.
- Answer schema validates strict shape, response-type compatibility, bounds, and no pre-submit
  answer-key leak. Existing provenance/grounding/canonical-rubric gates remain live.
- Practice/Exam counts and Module selection still work for mixed question types.

### Required validation

- Go fmt/build/vet/unit, fresh disposable migrations plus integration tests.
- Web unit/typecheck/build and authenticated Playwright mixed-type flow.
- Strict shared TypeScript, Compose config, Python tests, `git diff --check`.

### Stop condition

Write handoff and stop at review. Independent security/behavior review before release.
