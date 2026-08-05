# Active tasks

## T-0011 — Biology vertical-slice scope realignment

- Status: `review`
- Type: migration + code/tests + documentation
- Context: Product scope replaces Cambridge O Level Biology 5090 with Cambridge International AS & A Level Biology 9700 while preserving 5090 history and all provenance/content records.
- Goal: Make 0610 and one metadata-only 9700 catalogue syllabus active, retire existing 5090 catalogue row without deletion or downstream content mutation, and align project documentation.
- Scope:
  - Add one additive, idempotent Core migration.
  - Review fix: make 0016's 9700 conflict path `DO NOTHING`, preserving later human catalogue edits
    when historical SQL is directly re-executed.
  - Add migration/integration coverage for active resolution, retirement preservation, and absence of seeded 9700 content.
  - Update project, curriculum, roadmap, and local-setup documentation naming active Biology scope.
  - Produce review handoff and one T-0011 commit; do not push.
- Allowed files:
  - `services/core/migrations/0016_realign_biology_vertical_slice.sql`
  - `services/core/internal/**` test files genuinely needed for migration/catalogue resolution coverage
  - `CLAUDE.md`
  - `README.md`
  - `docs/**`
  - `product/**`
  - shared contracts only if runtime contract change proves necessary
- Forbidden files:
  - Any local Coursebook PDF or derived/extracted material
  - Existing migrations 0001–0015
  - Content-source approval/link/event data and curriculum node/question/rubric records or seeds
  - Unrelated user files and untracked workspace artifacts
- Non-goals:
  - No 9700 content source, curriculum node, question, rubric, syllabus text, objectives, questions, diagrams, past papers, mark schemes, OCR, ingestion, or derivative content.
  - No split AS/A Level catalogue rows; no content-source association changes.
  - No production/staging mutation, deployment, push, or shared-contract churn without need.
- Plan-first answers:
  - Before: 0610 and 5090 catalogue rows active; existing content-source history remains independent.
  - After: 0610 and exactly one 9700 row active; existing 5090 row retained as retired.
  - Fresh and initialized databases use same additive migration; rerun must be safe.
  - Rollback is intentionally not destructive; migration preserves all rows and references.
- Acceptance criteria:
  - Fresh database has active 0610/9700 and retired 5090.
  - Migration rerun is idempotent.
  - Direct 0016 re-execution preserves existing 9700 ID, timestamps, human-edited catalogue
    metadata, events, and dependent-record counts.
  - Active content-source association lookup resolves 9700 and rejects 5090.
  - Existing 5090 row remains present with identity/history intact.
  - No 9700 source/node/question/rubric content is seeded or existing downstream data changed.
  - Required Core, web, Python, shared-contract, Compose, and diff checks pass.
- Validation commands: Docker Go build/vet/gofmt/unit tests; disposable `sidus-test` fresh migration, rerun, integration tests, `down -v`; web Vitest/typecheck/build; Python tests; strict shared-contract TypeScript; dev/test Compose config; `git diff --check`.
- Security/privacy: Metadata only. Never inspect or derive from local Coursebook PDFs. Future 9700 sources require human-verified rights/provenance through editorial source registry.
- Review checklist: additive migration only; exact seed cardinality; retirement without deletion; no downstream mutation; tests cover positive/negative paths; docs consistently distinguish active and historical scope; independent review required before `done`.
- Handoff: `docs/handoffs/T-0011.md` records files, migration behavior, exact commands/results, blockers, protected-file status, and commit.
- Stop condition: Leave status `review` after implementation, validation, handoff, and commit. Never mark `done`; never push.
