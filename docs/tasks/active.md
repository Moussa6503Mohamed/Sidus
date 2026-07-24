# Active tasks

## T-0003 — Clerk authentication and roles foundation

**Status:** review
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done), T-0002 (done)

### Goal

Clerk owns authentication; Sidus Core owns authorization. No user-controlled `actorId` or
`reviewerId`. Audit identity (event actor, review reviewer) derives only from the verified
Clerk session subject.

### Scope

**Web (`apps/web`, Next.js 16 / React 19):**

- Install current compatible `@clerk/nextjs`.
- `ClerkProvider` in the app; Clerk proxy/middleware (`proxy.ts`) via Next.js 16 conventions.
- Sign-in and sign-up routes using Clerk components; no custom password handling.
- Health endpoint stays public. Protected placeholder `/dashboard` route.
- Home page: signed-out → sign-in/sign-up; signed-in → user menu + dashboard link.

**Core (`services/core`, Go):**

- Clerk JWT verification middleware using the official Clerk Go SDK / JWKS verification.
- Validate expiry, signature, issuer, authorized party. Allowed origins from env; dev
  default only the local Sidus origin.
- Every content-source endpoint requires a valid Clerk session; health stays public.
- Extract Clerk subject as authenticated actor; remove `actorId`/`reviewerId` from bodies.
- Role authorization from verified `sidus_role` claim:
  - `learner`: no content-source access
  - `editor`: create + PATCH pending sources
  - `reviewer`: editor + approve/reject
  - `admin`: all content-source permissions
  - missing/unknown role: deny by default
- Verified subject stored as event actor / review reviewer.
- `401` for missing/invalid token, `403` for valid token lacking permission.
- No Clerk Backend API call per request — verified session JWT claims + JWKS caching.

**AI (`services/ai`, FastAPI):**

- Auth dependency/foundation verifying the Clerk bearer token.
- Ingestion rights gate unchanged. Protect a future ingestion route foundation; no
  OCR/content ingestion created.

**Contracts / docs:**

- Shared contracts updated for authenticated requests; caller-controlled actor/reviewer
  fields removed. Auth env placeholders only.
- Document Clerk Dashboard manual setup, local auth setup, token flow, backend
  verification, and the role matrix.

### Environment safety

- Real Clerk keys live only in ignored `.env.local` files. Never print, log, commit, stage,
  copy into docs, or expose any key. `.env.example` holds placeholders only.
- `apps/web/.env.local` (ignored) may be created/updated for local runtime; never staged.
- Preserve all untracked user files, `.claude/`, `.claude-flow/`.

### Assumptions / decisions

- Recorded in `docs/decisions.md` (D-0006) and the handoff as work proceeds.

### Open questions / blockers

- Recorded here if a missing answer changes scope, data model, security, rights, or cost.

### Handoff

`docs/handoffs/T-0003.md`
