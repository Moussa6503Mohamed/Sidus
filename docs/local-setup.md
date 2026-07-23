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
cd services/core && DATABASE_URL="postgres://sidus:sidus_dev_password@localhost:5432/sidus?sslmode=disable" go run ./cmd/migrate
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
cd services/core && go test ./...
# Postgres-backed integration test (optional, needs the database running):
cd services/core && TEST_DATABASE_URL="postgres://sidus:sidus_dev_password@localhost:5432/sidus?sslmode=disable" go test ./... -run Integration
cd services/ai && python -m pytest
```
