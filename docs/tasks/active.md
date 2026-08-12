# Active tasks

## T-0035 — Learning analytics foundation

**Status:** review
**Type:** Core data + learner projection + dashboard UI + tests.

### Context

Practice, Exam, assessment sessions, deterministic marking, and automated written marking already
produce verified learner outcomes. Sidus needs learner-owned progress views without exposing answer
keys, source provenance, other learners, or unfinalized AI data.

### Goal

Create durable learner analytics from completed attempts and marking outcomes: subject/module
performance, pending/withheld visibility, recent activity, and safe dashboard summaries.

### Scope

- Add append-only learning analytics events and reproducible learner-owned aggregates.
- Include deterministic outcomes immediately and accepted automated written outcomes only after
  final marking; pending/withheld outcomes remain explicitly unscored.
- Provide fixed Core learner routes, closed BFF operations, shared learner-safe contracts, and
  branded dashboard/progress UI.
- Preserve history under question/rubric/module changes; no cross-learner, editorial, source,
  canonical-answer, raw-answer, provenance, cost, or model-trace exposure.

### Implementation plan / assumptions

- Persist only owner-bound, outcome-only snapshots at the existing attempt finalization boundary:
  deterministic submissions are scored immediately; written submissions start pending and gain a
  separate terminal accepted/withheld event only after the immutable marking lifecycle settles.
- Rebuild aggregates from those append-only snapshots rather than current question/rubric/module
  records, so lifecycle and label changes cannot rewrite a learner's history.
- Use one learner-safe summary endpoint and a fixed BFF operation. No user-supplied filters,
  teacher scope, ranking, or source-linked drill-down is required for this foundation.

### Allowed files

- `services/core/internal/learner/**`, `services/core/main.go`,
  `services/core/migrations/0025_*`, Core tests.
- `apps/web/app/dashboard/**`, `apps/web/app/api/learner/**`, `apps/web/lib/learner/**`, web tests/styles.
- `packages/shared/src/contracts.ts`, `docs/learning-analytics.md`, `docs/decisions.md`,
  `docs/handoffs/T-0035.md`, `CLAUDE.md`, this task file.

### Forbidden

- Live AI/API calls, PDFs/OCR/extraction/source reads, content seeds, human reviewer workflows,
  notifications, rankings/social gamification, teacher/class analytics, or cross-user access.
- Raw answers, source/provenance, canonical keys, rubric descriptors, model traces/costs in learner
  routes or browser logs.
- Protected/untracked files and unrelated refactors.

### Acceptance criteria

- Events and aggregates are owner-scoped, append-only, idempotent, and safe under retry/races.
- Score denominator reflects all completed eligible items; pending/withheld marking is visibly
  separate and cannot inflate/deflate scored aggregates.
- Module and syllabus summaries remain available after lifecycle changes without leaking private
  data; no learner may query another learner.
- Dashboard shows loading, empty, partial/pending, error/retry, and populated states using Sidus
  white-primary Observatory tokens.
- Strict IDs/query validation occurs before authentication/upstream calls. No API/config means no
  fabricated analytics.

### Validation

- Core build/vet/unit; fresh disposable migration/integration/rerun.
- Web tests/typecheck/build; strict shared TypeScript; both Compose configs; diff/secret audit.
- No live AI/API or private-source testing.

### Review checklist

- Owner isolation, event immutability, idempotency/races, denominator correctness, pending/
  withheld handling, snapshot/history safety, learner projection, UI state/a11y, no private leaks.

### Stop condition

Implementation committed, independent review approved, release validation passed, release docs
committed/pushed. Stop before T-0036.
