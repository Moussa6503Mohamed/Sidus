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

See `infra/README.md` for details. `services/core` only mounts the `/content-sources` endpoints when `DATABASE_URL` is set.

## Run

```sh
cd apps/web && npm install && npm run dev
cd services/core && DATABASE_URL="postgres://sidus:sidus_dev_password@localhost:5432/sidus?sslmode=disable" go run .
cd services/ai && python -m venv .venv && .venv/Scripts/pip install -r requirements.txt && .venv/Scripts/uvicorn app.main:app --reload
```

## Check

```sh
cd apps/web && npm run typecheck
# No host Go toolchain required — run go test via the golang Docker image:
docker run --rm -v "$(pwd)/services/core:/app" -w /app golang:1.22-alpine go test ./...
# Postgres-backed integration test (optional): needs the disposable postgres-test service from
# docker-compose.test.yml, NEVER the dev postgres above. See docker-compose.test.yml usage comment.
docker run --rm --network sidus-test_default -v "$(pwd)/services/core:/app" -w /app \
  -e TEST_DATABASE_URL="postgres://sidus_test:sidus_test_password@postgres-test:5432/sidus_test?sslmode=disable" \
  golang:1.22-alpine go test ./... -run Integration
cd services/ai && python -m pytest
```
