# Local setup

## Prerequisites

- Node.js 20+
- Go 1.22+
- Python 3.11+

## Run

```sh
cd apps/web && npm install && npm run dev
cd services/core && go run .
cd services/ai && python -m venv .venv && .venv/Scripts/pip install -r requirements.txt && .venv/Scripts/uvicorn app.main:app --reload
```

## Check

```sh
cd apps/web && npm run typecheck
cd services/core && go test ./...
cd services/ai && python -m pytest
```
