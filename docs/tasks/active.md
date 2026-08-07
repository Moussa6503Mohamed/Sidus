# Active tasks

## T-0021 — Private licensed-source references (D-0021)

**Status:** review
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0009 (done, editorial source workflow)

### Context

User-supplied task context references "T-0020 approved source granularity: one
`content_sources` record per matched QP/MS pair" and a private manifest at
`D:\Sidus-private-content\manifests\biology-9700-source-manifest.csv` (489 eligible 9700 QP/MS
pairs). **No T-0019/T-0020 task or decision exists in this repo's `docs/tasks/history.md` or
`docs/decisions.md`** — highest prior task is T-0018, highest prior decision is D-0020. The
private-content repo's own intake report (`biology-9700-source-intake-report.md`) independently
confirms the QP/MS pairing concept and the `{session}{2-digit-year}_{qp|ms}_{component}` filename
shape (e.g. `9700_m17_qp_12.pdf`), which matches the `{session}`/`{component}` grammar given for
D-0021. This task's own scope is fully self-contained (a validator addition only — it creates no
source rows, seeds nothing, and changes no granularity/approval rule), so the missing T-0019/T-0020
repo record does not change this task's scope, data model, security, rights, or cost. Recorded here
as an open question rather than a blocker.

### Goal

Let a pending `content_sources.sourceUrl` accept a tightly validated private reference URI for
licensed 9700 QP/MS pair bundles, without weakening existing HTTP/HTTPS validation, and without
touching any private source file or seeding any runtime data.

### Scope

1. `services/core/internal/contentsource`: one strict `sourceUrl` validator used by both create
   and update (create currently has no format check at all; update's `isValidHTTPURL` becomes
   shared). Accepts absolute `http`/`https` URLs (unchanged behavior) or exactly
   `sidus-private://licensed/cambridge-international/9700/{session}/{component}` where
   `{session}` matches `[msw]\d{2}` and `{component}` is exactly two digits — no paths, drive
   letters, credentials, query, fragment, port, alternate host, percent-encoding, or other
   private scheme.
2. `packages/shared/src/contracts.ts`: document `sourceUrl` as external URL or approved private
   source-reference URI (comment only — the field stays `string`).
3. `apps/web/app/dashboard/editorial/sources/source-form.tsx`: help text for the private URI
   form. No learner surface change.
4. Docs: add D-0021 to `docs/decisions.md`; note the private URI form in
   `docs/editorial-source-workflow.md` and `docs/content-provenance-register.md`; this file and
   `docs/handoffs/T-0021.md`.
5. Tests: Go create/update matrix (valid HTTP(S), valid private URI, every rejected variant);
   web BFF/form coverage confirms no client-side change weakens Core's authority; confirm learner
   projections never carry `sourceUrl` (already true — verified by inspection of
   `services/core/internal/learner`, no new test needed unless inspection finds a gap).

### Forbidden (restated from instruction)

No read/render/OCR/hash/copy/move/upload of files under `D:\Sidus-private-content\licensed`. No
source/approval/catalogue-link/map/question/rubric/attempt/runtime row creation. No rights-gate
rule change. No unrelated auth/learner/AI/migration/Docker/dependency/secret change. Never stage
`.claude/`, `.claude-flow/`, images, spreadsheets, PDFs, ZIPs, `.env.local`.

### Acceptance criteria

- Create and update reject every malformed/case-variant/unsafe `sourceUrl` before any store call,
  with the existing `invalid_source_url` error code (no new error code invented).
- Valid `http`/`https` URLs behave exactly as before (no regression).
- The exact D-0021 private URI form is accepted on create and update.
- Learner projections (`LearnerQuestion`, `LearnerSyllabus`, attempt types) never expose
  `sourceUrl` — confirmed, not assumed.
- No schema/migration change (`source_url` column has no format CHECK constraint today).

### Validation commands

- `npm --prefix apps/web run typecheck && npm --prefix apps/web run test && npm --prefix apps/web run build`
- `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts`
- `docker run --rm -v "$(pwd)/services/core:/app" -w /app golang:1.22-alpine sh -c "gofmt -l . && go build ./... && go vet ./... && go test ./..."`
- Disposable `sidus-test` Postgres: fresh migrate, rerun migrate (idempotent), integration tests, teardown.
- `cd services/ai && python -m pytest -q`
- `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config`
- `git diff --check`
- Staged-content/secret/protected-file audit before commit.

### Open questions / blockers

- T-0019/T-0020 are referenced by the task instruction as prior context but do not exist in this
  repo's task/decision log (see Context above). Not blocking this task; flagged for the user to
  reconcile numbering/history if those steps should be recorded separately.

### Handoff

`docs/handoffs/T-0021.md`.
