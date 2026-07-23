# Infrastructure placeholders

Local PostgreSQL is provisioned: see `docker-compose.yml` at repo root and `services/core/migrations/`. Future local development services: Redis, S3-compatible object storage, OpenSearch, and worker queues. Add Compose manifests for those only once service configuration exists.

## PostgreSQL

```sh
cp .env.example .env   # override POSTGRES_* / POSTGRES_PORT if 5432 is taken locally
docker compose up -d postgres
```

Migrations live in `services/core/migrations/*.sql` (numbered, idempotent) and are applied by the `services/core/cmd/migrate` Go tool, which tracks applied files in a `schema_migrations` table:

```sh
cd services/core
DATABASE_URL="postgres://sidus:sidus_dev_password@localhost:5432/sidus?sslmode=disable" go run ./cmd/migrate
```

Rerunning `go run ./cmd/migrate` is safe: already-applied files are skipped.
