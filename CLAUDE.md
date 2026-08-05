# Sidus agent guide

Read before planning, editing, testing, or committing.

## Mission

Build Sidus: academic preparation platform. First vertical slice: Cambridge IGCSE Biology 0610 Extended and Cambridge O Level Biology 5090.

## Read order

1. `docs/tasks/active.md`
2. `docs/decisions.md`
3. `docs/agent-workflow.md`
4. Relevant architecture and content documents

## Architecture

- `apps/web`: Next.js + TypeScript PWA.
- `services/core`: Go high-traffic API.
- `services/ai`: Python/FastAPI AI, OCR, ingestion, marking.
- `packages/shared`: cross-service contracts.
- PostgreSQL is system of record. Redis/object storage/OpenSearch come later.
- Anthropic only: Haiku routine tasks; Sonnet complex marking.
- Canonical explanation cache key: `question + syllabus + rubric + language + explanation version`.

## Non-negotiable content rules

- Never commit PDFs, books, extracted text, diagrams, screenshots, past papers, mark schemes, or lightly rewritten questions.
- Use only source metadata and reviewed mappings until rights approval exists.
- Content ingestion blocks every source except `approved`.
- Original questions link to syllabus/objective IDs, not copied source wording.

## Working rules

- Do not guess. Record missing detail under `Open questions` or `Blockers` in active task.
- Work only task scope. Create a new task entry before scope expansion.
- Preserve unrelated files and user changes. Stage only own files.
- Run relevant checks. Record command and result in task handoff.
- Update task status and create handoff before commit/hand-off.
- Never overwrite another agent's active task. Ask user or create separate task.

## Commands

```sh
npm --prefix apps/web run typecheck
npm --prefix apps/web run test
npm --prefix apps/web run build
docker run --rm -v "$(pwd)/services/core:/app" -w /app golang:1.22-alpine go test ./...
cd services/ai && python -m pytest
```

## Current state

- Foundation commit: `e7e2179`.
- Biology syllabus/provenance commit: `4cfb5d3`.
- Curriculum catalogue (multi-subject) released in T-0004. Core owns the metadata-only
  `subjects`/`syllabuses` registry; see `docs/curriculum-catalogue.md` and D-0007.
- Provenance-confirmed catalogue linking released in T-0005; see
  `docs/provenance-catalogue-linking.md`. Two seeded 0610/5090 content sources still need a
  human editor/admin `PATCH` to link (documented there).
- Curriculum-map foundation (metadata-only topic/objective/practical-skill/assessment-rule
  infrastructure) released in T-0006; see `docs/curriculum-map.md` and D-0008. No map data is
  seeded — a human must first approve and link a content source, then author map content via a
  future private approved workflow. Four review findings were fixed on top of `b1677cb`
  (strict PATCH decoding, source gate re-validated on every node write, syllabus validation on
  map list, real ancestor row locking) — see D-0008 "Update (T-0006 review)".
- Original question and versioned rubric foundation (private, metadata/code-only infrastructure
  for a future Exam Mode) built in T-0007; see `docs/question-rubric-model.md` and D-0009. Core
  owns `questions`/`question_rubric_versions`/`question_events` (migrations 0012–0015). Every
  question must trace to a **verified** curriculum-map node whose approved source still passes the
  T-0006 source gate — re-checked on every write. No question, rubric, or map data is seeded, and
  no AI/OCR/ingestion path is involved. Three review findings were fixed on top of `768c8e2`
  (question `content_revision` so a question can only be verified with a rubric verified against
  its **current** content, case-sensitive rubric JSON keys, validated node filter on
  `GET /questions`) — see D-0009 "Update (T-0007 review)". Released 2026-08-05.
- Cross-package API input hardening released in T-0008: `contentsource`, `catalogue`, and
  `curriculummap` now match `question`'s strict-decode allowlist (rejects case-variant field
  names, `actorId`/`reviewerId`, unknown fields, trailing JSON, and literal JSON `null` bodies)
  and map malformed (non-UUID) `{id}` path parameters to the existing not-found response instead
  of a `500` — see D-0010 and `docs/handoffs/T-0008.md`. No business rule, schema, or seed
  content changed. Released 2026-08-05.
- Private editorial source workflow released in T-0009: `apps/web` gains a protected
  `/dashboard/editorial/sources` page and a narrow BFF (`app/api/editorial/*`, backed by
  `lib/editorial/core-proxy.ts`) that is the browser's only path to the content-source/
  catalogue APIs — see `docs/editorial-source-workflow.md` and D-0011. Fixed operation
  allowlist (no open proxy), server-only `SIDUS_CORE_API_URL` and Clerk bearer forwarding,
  fail-closed on missing config/token. Core stays the sole authorization authority; the web
  role check only hides/shows controls. No Core/AI/migration/business-rule change, and no
  approval/link of the seeded 0610/5090 sources was performed (that is now a human action
  through the new UI). A review fix on top of `15d5936` (commit `f31a8a5`) sanitizes any Core
  `5xx` response to a generic `502` before it reaches the browser and sets the upstream `fetch`
  to fail closed on redirects (`redirect: "error"`) — see D-0011 "Update (T-0009 review)" and
  `docs/handoffs/T-0009.md`. T-0009 released and moved to `docs/tasks/history.md` as `done`.
- No active task. See `docs/tasks/active.md` / `docs/tasks/history.md`.
