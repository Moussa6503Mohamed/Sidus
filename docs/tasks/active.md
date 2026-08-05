# Active tasks

## T-0012 — Private editorial question and rubric workflow

**Status:** review
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0007 (done), T-0009 (done), T-0010 (done)

### Context

Core T-0007 owns original questions, content revisions, grounding/source gates, lifecycle,
versioned rubrics, and authorization. Browser has no private editorial workflow for those
existing operations.

### Goal

Add secure same-origin Next.js editorial workflow for authoring and reviewing original questions
and versioned rubrics while keeping Core sole authority and creating or changing no runtime data
automatically.

### Task type

Code + tests + UI + minimal Core read-contract correction + documentation + release validation.

### Scope

- Extend closed `EditorialOperation` union and add fixed `/api/editorial/questions/*` handlers for
  existing Core question/rubric operations.
- Add protected `/dashboard/editorial/questions` workflow with syllabus and verified-node
  selection, draft question editing, rubric-version workspace, lifecycle review actions, safe
  error states, and mobile-safe existing styles.
- Apply minimal Core read correction required for editorial discovery: authorized question reads
  may return any lifecycle status; list supports validated `draft`/`verified`/`retired`/`all`.
- Add focused web/Core tests and T-0012 workflow, decision, setup, protocol, and handoff docs.

### Allowed files

- `apps/web/app/api/editorial/questions/**`
- `apps/web/app/dashboard/editorial/questions/**`
- `apps/web/app/layout.tsx`
- `apps/web/lib/editorial/core-proxy.ts` and its tests
- `services/core/internal/question/**` only where required for read-contract correction/tests
- `packages/shared/src/contracts.ts` only if actual Core contract requires alignment
- `docs/tasks/active.md`, `docs/decisions.md`, `docs/local-setup.md`,
  `docs/editorial-question-rubric-workflow.md`, `docs/handoffs/T-0012.md`, `CLAUDE.md`

### Forbidden files

- `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx`, `.env.local`, PDFs, ZIPs,
  design bundles, migrations, seeds, AI service business logic, source/PDF/extract/diagram/
  syllabus/past-paper/mark-scheme content.

### Non-goals and boundaries

- No automatic create, seed, verify, retire, or alteration of runtime questions, rubrics, sources,
  or nodes.
- No Core write-rule, grounding, source-gate, content-revision, rubric-versioning, lifecycle,
  authorization, AI, or migration change.
- No open proxy, browser-visible Core URL/token, major redesign, or sample question/rubric content
  in repository tests, defaults, fixtures, docs, or seeds.

### Plan-first questions

- Core route inspection confirms existing paths exactly: `/questions`, `/questions/{id}`,
  `/questions/{id}/verify`, `/questions/{id}/retire`,
  `/questions/{id}/rubric-versions`, and
  `/questions/{id}/rubric-versions/{version}/verify`.
- Resolved from user authorization: minimal Core read-contract correction is allowed because
  current verified-only list/get makes required draft editorial workflow impossible. Mirror
  T-0010 pattern; no write-side or permission change.

### Acceptance criteria

- Browser calls only fixed `/api/editorial/*` routes; BFF validates ids, syllabus/node query,
  lifecycle filter, JSON content type/size/syntax before Core fetch.
- Missing Core URL returns 503; missing token 401; invalid route/query input 400 before fetch;
  Core 5xx returns generic 502; redirects fail closed; no secret/body/token/Core URL logging.
- Editor/reviewer/admin can access workflow; reviewer/admin alone see verify/retire actions;
  learner/unknown get denied state and zero API calls. Core remains authorization authority.
- UI covers syllabus, verified-node selection, question list/form, rubric versions/criteria,
  revision freshness, confirmations, safe domain errors, loading/empty/retry/denied/mobile states.
- No prohibited or sample question/rubric content added; existing T-0009/T-0010 tests stay green.

### Validation commands

- `npm --prefix apps/web run test`
- `npm --prefix apps/web run typecheck`
- `npm --prefix apps/web run build`
- strict TypeScript over `packages/shared/src/contracts.ts`
- Dockerized Go build, vet, and unit tests
- disposable `sidus-test` fresh migration, idempotent rerun, integration tests, teardown
- Python tests
- dev/test Compose config validation
- `git diff --check`

### Security/privacy constraints

- Server-side Clerk token forwarding only; fixed Core operation union/paths only.
- Validate caller-controlled path/query/body envelope before network work.
- Preserve generic upstream failure handling and redirect refusal; never log secrets or content.
- Prompts/descriptors exist only from explicit private human runtime input, never repository data.

### Review checklist and handoff requirements

- Confirm route export allowlist, fixed Core mappings, pre-fetch validation, role gates, UI states,
  content-safety scan, no runtime writes, protected-file status, and full validation results.
- Add `docs/handoffs/T-0012.md`; leave task `review`; one T-0012 commit; do not push.
- Independent review required before task may become `done`.

### Stop condition

Stop at `review` after implementation, validation, handoff, and one local commit. Do not push.

### Review handoff

Implementation and required validation complete. See `docs/handoffs/T-0012.md` and D-0014.
Independent review remains required before `done`.
