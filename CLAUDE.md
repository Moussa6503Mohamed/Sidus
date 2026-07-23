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
cd services/core && go test ./...
cd services/ai && python -m pytest
```

## Current state

- Foundation commit: `e7e2179`.
- Biology syllabus/provenance commit: `4cfb5d3`.
- Active task: `T-0001` rights/provenance gate. See `docs/tasks/active.md`.
