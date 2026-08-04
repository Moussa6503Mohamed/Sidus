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
  infrastructure) built in T-0006; see `docs/curriculum-map.md` and D-0008. No map data is
  seeded — a human must first approve and link a content source, then author map content via a
  future private approved workflow. Four review findings were fixed on top of `b1677cb`
  (strict PATCH decoding, source gate re-validated on every node write, syllabus validation on
  map list, real ancestor row locking) — see D-0008 "Update (T-0006 review)".
- Active task: T-0006 (status `review`). See `docs/tasks/active.md`.
