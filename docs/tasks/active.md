# Active tasks

## T-0009 — Private editorial source workflow

**Status:** review
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0003 (done), T-0004 (done), T-0005 (done), T-0008 (done)

### Goal

First secure web-to-Core workflow for editorial staff: source registry → metadata completion →
catalogue syllabus association → rights review → approval/rejection. This unlocks human
approval/linking for the seeded 0610/5090 sources (T-0001/T-0005) — it does not perform those
approvals or links itself.

### Scope

- Protected `/dashboard/editorial/sources` page in `apps/web`, gated to `editor`/`reviewer`/
  `admin` (server-side role check from the verified Clerk session; `learner`/unknown see no
  editorial controls).
- Narrowly scoped Next.js route handlers (BFF) that proxy only allowlisted Core endpoints:
  `GET/POST /content-sources`, `GET/PATCH /content-sources/{id}`,
  `POST /content-sources/{id}/approve`, `POST /content-sources/{id}/reject`,
  `GET /catalogue/syllabuses`. No open proxy — fixed Core base URL from server-only
  `SIDUS_CORE_API_URL`, explicit per-route operation, no caller-controlled target URL.
- Fail closed: missing `SIDUS_CORE_API_URL` or missing Clerk session token → safe
  unavailable/unauthorized JSON response, never a leaked secret or raw upstream error.
- Core stays the sole authorization authority — the web role check only hides/shows UI
  controls; every mutation is still enforced (401/403) by Core's existing `auth.Protect`.
- No curriculum-map or question/rubric authoring UI (out of scope).
- No content ingestion, OCR, AI generation, or copyrighted material of any kind.

### Assumptions

- `SIDUS_CORE_API_URL` is a new server-only env var (never `NEXT_PUBLIC_*`), analogous to how
  Core already gates on `DATABASE_URL`/Clerk config (`services/core/main.go`).
- The web app has no existing test runner; Vitest + Testing Library is added (dev deps only,
  package-lock changes expected and are in scope for this task).
- `packages/shared` is not yet wired into `apps/web`'s module resolution; a `tsconfig.json`
  `paths` entry is added so it resolves without npm workspaces (no root `package.json` exists).
- UI role visibility reads the verified `sidus_role` Clerk session claim server-side
  (`sessionClaims`, populated by Clerk from the signed token — not client-suppliable); this is
  cosmetic only, per D-0006/D-0009 precedent that Core is the sole authorization authority.

### Open questions

None blocking. Manual human approval/linking of the seeded 0610/5090 sources through this new
UI is intentionally left to a human editor/admin after this task lands (matches T-0005's
existing carried-forward note).

### Plan

1. Web test tooling (Vitest/RTL) + `tsconfig.json` path aliases (`@/*`, `@sidus/shared`).
2. `lib/editorial/*`: Core-proxy engine (fixed operation allowlist, id validation, timeouts,
   fail-closed checks, no logging), role/permission helpers.
3. Route handlers under `app/api/editorial/*` calling the proxy engine only.
4. Route-handler tests (mocked Clerk/Core).
5. `/dashboard/editorial/sources` page + client workspace components (list, create/edit form,
   review actions, status indicators, loading/empty/error states); gated nav entry in
   `app/layout.tsx`.
6. Component tests.
7. Docs: `docs/editorial-source-workflow.md`, `docs/handoffs/T-0009.md`, `docs/decisions.md`
   (new D-0011), `docs/local-setup.md`, `.env.example`, `CLAUDE.md`.
8. Full validation (web + Go + Python + shared TS + compose + git diff --check), one commit,
   status left at `review`.

See `docs/tasks/history.md` for completed tasks.
