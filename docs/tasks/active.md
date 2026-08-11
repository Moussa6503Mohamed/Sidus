# Active tasks

## T-0033 — Sonnet adapter and automated quality gate

**Status:** review
**Type:** AI contract + job orchestration + evaluation + tests.

### Implementation summary

`services/ai/app/sonnet/` adds: strict pydantic request/result schemas (`schemas.py`), a
`SonnetProvider` protocol with a fail-closed `get_provider()` that always returns `None` in this
build (`provider.py`), a deterministic `FakeSonnetProvider` for tests (`fake_provider.py`), a
rubric/consistency quality gate (`quality_gate.py`), a SQLite-backed durable `JobStore` with an
append-only per-attempt trace (`jobs.py`), a retry/withhold orchestrator (`orchestrator.py`), and
Clerk-protected, owner-scoped `POST /sonnet/jobs` / `GET /sonnet/jobs/{id}` routes (`routes.py`),
wired into `app/main.py`. 42 new tests pass alongside the 20 pre-existing AI-service tests (62
total). See `docs/decisions.md` D-0026 and `docs/handoffs/T-0033.md`.

### Goal

Prepare fail-closed Anthropic Sonnet marking/review integration without requiring a live API key.

### Scope

- Provider interface, strict request/result schemas, version/cost/confidence trace, job lifecycle,
  retry/withhold behavior, and automated rubric/consistency quality gates.
- AI service endpoints and Core integration boundary. Model output may never auto-create/publish
  learner content outside existing verification gates.

### Boundaries

- No API key, live Anthropic call, source PDF access, OCR, extraction, question rewriting, or
  content seeding. Tests use deterministic fake provider only.

### Stop condition

Stop at review. Live API tests remain deferred until API configuration phase.
