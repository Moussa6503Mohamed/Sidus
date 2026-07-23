# Sidus

Sidus is an academic preparation platform. This repository holds product foundation only: code, contracts, architecture, and metadata-only resource inventory.

## Vertical slice

First target: Cambridge IGCSE Biology 0610 Extended and Cambridge O Level Biology 5090.

## Services

- `apps/web` — Next.js frontend/PWA foundation.
- `services/core` — Go core API foundation.
- `services/ai` — Python/FastAPI AI and ingestion foundation.
- `packages/shared` — shared TypeScript contracts.
- `infra` — local infrastructure placeholders.
- `docs` — architecture, setup, content inventory, gap reports.

## Quick start

See [local setup](docs/local-setup.md). Health endpoints:

- Web: `GET /api/health`
- Core: `GET /healthz`
- AI: `GET /healthz`

## Content safety

Do not commit copyrighted books, PDFs, extracted text, diagrams, questions, or derived content. Store only approved metadata and independently authored material with documented rights.
