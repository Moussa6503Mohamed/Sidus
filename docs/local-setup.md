# Local setup

## Prerequisites

- Node.js 20+
- Go 1.22+
- Python 3.11+
- Docker (for local PostgreSQL)

## Database

```sh
cp .env.example .env   # override POSTGRES_* / POSTGRES_PORT if 5432 is taken locally
docker compose up -d postgres
# No host Go toolchain required — run via the golang Docker image:
docker run --rm --network sidus_default -v "$(pwd)/services/core:/app" -w /app \
  -e DATABASE_URL="postgres://sidus:sidus_dev_password@postgres:5432/sidus?sslmode=disable" \
  golang:1.22-alpine go run ./cmd/migrate
```

See `infra/README.md` for details. `services/core` mounts the `/content-sources`,
`/catalogue` (curriculum catalogue — see `docs/curriculum-catalogue.md`),
`/curriculum-map` (see `docs/curriculum-map.md`), **and** `/questions` (original questions and
versioned rubrics — see `docs/question-rubric-model.md`) endpoints only when
`DATABASE_URL` is set **and** Clerk is safely configured — `CLERK_SECRET_KEY` and
`CLERK_JWT_ISSUER` present and non-blank, and `CLERK_AUTHORIZED_PARTIES` either absent (dev
default `http://localhost:3000`) or with at least one valid origin. A missing issuer or an
explicitly blank authorized-parties list keeps the routes disabled (fail closed — no
unauthenticated or unrestricted content access). The AI service's protected routes return
`503` under the same misconfiguration. See `docs/auth-setup.md` → "Fail-closed configuration".

After fresh migration, active Biology catalogue rows are 0610 Extended and one combined 9700
International AS & A Level row. Historical 5090 remains present with `retired` status. Migration
creates no 9700 source or curriculum/question/rubric content.

Migration 0017 adds nullable `questions.options` JSONB only. It is safe on existing databases and
rerunnable; existing rows remain `NULL`. Core, not migration SQL, enforces MCQ option and rubric
answer-key rules. No question, option, rubric, answer key, or other content is seeded.

Migration 0018 adds nullable canonical-rubric FK and partial index. It is additive/rerunnable and
does not backfill historical verified questions or select any rubric. Reviewer/admin performs
selection explicitly through editorial question workflow.

## Authentication (Clerk)

Clerk owns authentication; Sidus Core owns authorization. Set up keys and the `sidus_role`
claim per `docs/auth-setup.md`. Real keys live only in **gitignored** `.env.local` files —
never in `.env.example`. For the web app create `apps/web/.env.local` with your Clerk keys.

## Run

```sh
cd apps/web && npm install && npm run dev
cd services/core && DATABASE_URL="postgres://sidus:sidus_dev_password@localhost:5432/sidus?sslmode=disable" go run .
cd services/ai && python -m venv .venv && .venv/Scripts/pip install -r requirements.txt && .venv/Scripts/uvicorn app.main:app --reload
```

To use editorial source (`/dashboard/editorial/sources`, T-0009), curriculum-map
(`/dashboard/editorial/curriculum`, T-0010), or question/rubric
(`/dashboard/editorial/questions`, T-0012–T-0014, including draft MCQ options, answer-key editing,
and explicit canonical-rubric selection) workflow, also add
`SIDUS_CORE_API_URL=http://localhost:8080` to `apps/web/.env.local` (server-only — never
`NEXT_PUBLIC_*`; see `docs/editorial-source-workflow.md`,
`docs/editorial-curriculum-workflow.md`, and `docs/editorial-question-rubric-workflow.md` — all
workflows share the same env var and BFF layer). Sign in with a Clerk user whose
`sidus_role` public metadata is `editor`, `reviewer`, or `admin` per `docs/auth-setup.md`;
`learner`/unknown users see no editorial controls.

## Check

```sh
cd apps/web && npm run typecheck
cd apps/web && npm run test
# No host Go toolchain required — run go test via the golang Docker image:
docker run --rm -v "$(pwd)/services/core:/app" -w /app golang:1.22-alpine go test ./...
# Postgres-backed integration test (optional): needs the disposable postgres-test service from
# docker-compose.test.yml, NEVER the dev postgres above. See docker-compose.test.yml usage comment.
docker run --rm --network sidus-test_default -v "$(pwd)/services/core:/app" -w /app \
  -e TEST_DATABASE_URL="postgres://sidus_test:sidus_test_password@postgres-test:5432/sidus_test?sslmode=disable" \
  golang:1.22-alpine go test ./... -run Integration
cd services/ai && python -m pytest
```
