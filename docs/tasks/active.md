# Active tasks

## T-0028 — Learner module selection and availability-aware question count

**Status:** review
**Type:** Core/BFF/shared-contract/web UI + tests.
**Owner:** Codex.

### Context and goal

Replace opaque curriculum-node identifiers on learner Practice and Exam setup screens with
learner-safe module selection. Let learners choose any positive number of currently eligible
questions, or all available questions, in both modes. Learner UI must use "Module" only.

### Scope

- Add a learner-safe, fixed-route module-discovery read constrained to an active syllabus and
  modules with at least one currently eligible question.
- Add matching closed BFF operation and same-origin route.
- Add shared learner module contract; no editorial fields/source metadata/locators.
- Replace learner raw node-id fields in Practice and Exam with accessible module selects.
- Add question-count controls to both modes: positive integer or All, availability-aware, no
  arbitrary upper cap.

### Allowed files

- `services/core/internal/learner/**`
- `packages/shared/src/contracts.ts`
- `apps/web/lib/learner/**`
- `apps/web/app/api/learner/**`
- `apps/web/app/dashboard/practice/**`
- `apps/web/app/dashboard/exam/**`
- `apps/web/e2e/**` only if existing E2E needs selector update
- `docs/{tasks/active.md,handoffs/T-0028.md,learner-question-delivery.md,decisions.md}`
- `CLAUDE.md`

### Forbidden files / non-goals

- No private-source folders, PDFs, source text, question content, secrets, runtime source/node/
  question changes, migrations, AI, auth-policy changes, or response-type/marking expansion.
- No push/release. No learner disclosure of source locators/provenance/rubrics/answer keys.

### Plan-first questions

- Existing learner question eligibility remains sole availability authority; module discovery must
  expose only modules that currently contain at least one eligible question.
- Count stays client-side because Core list endpoint has no pagination/count contract; UI must
  validate before rendering/starting and never silently truncate.

### Acceptance criteria

- Learners can select syllabus then a human-readable Module or all modules; no learner UI text
  contains "node" or asks for opaque IDs.
- Module route/auth/BFF are fixed, fail-closed, and learner-safe.
- Practice and Exam accept any positive requested count up to availability and an All option;
  clear unavailable-count messaging; no 10 cap.
- Unit/integration/UI coverage proves normal/invalid/empty/access-denied paths and no learner
  metadata leak. Existing source/grounding/auth gates preserved.

### Validation

- Go fmt/build/vet/unit and disposable-DB integration.
- Web Vitest/typecheck/build; focused learner BFF/UI behavior.
- Shared strict TypeScript; compose config; diff check.

### Review / handoff / stop

- Document route, eligibility predicate, UI behavior, tests, and deferred response-type work in
  handoff. Stop at `review` after implementation and validation; independent review required
  before `done`.
