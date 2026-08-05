# Editorial source workflow (T-0009)

The first secure web-to-Core workflow for editorial staff: source registry → metadata
completion → catalogue syllabus association → rights review → approval/rejection. It is a
UI/BFF surface over the T-0001/T-0002/T-0003/T-0004/T-0005 content-source and catalogue APIs —
no new business rule, schema, migration, or seed content was added. It unlocks **human**
approval/linking of the seeded 0610/5090 sources; it does not perform those approvals or links
itself.

## Architecture

```
Browser  →  Next.js route handlers (app/api/editorial/*)  →  Sidus Core (services/core)
(fetch)     (same-origin, server-only)                        (sole authorization authority)
```

- **The browser never talks to Core directly.** Every editorial page (`/dashboard/editorial/
  sources`) and its client components call only same-origin Next.js route handlers under
  `app/api/editorial/*`.
- **The route handlers are a narrow proxy, not an open one.** `lib/editorial/core-proxy.ts`
  exposes a single `callCore(operation, rawBody?)` function. `operation` is a closed TypeScript
  union (`EditorialOperation`) — `listContentSources`, `createContentSource`,
  `getContentSource`, `updateContentSource`, `approveContentSource`, `rejectContentSource`,
  `listSyllabuses` — each mapped to exactly one fixed Core method+path template. There is no
  caller-controlled target URL: a route handler can select *which* allowlisted operation to
  invoke, never *where* it goes. A path `{id}` is validated (`^[A-Za-z0-9_-]{1,128}$`) before
  it is interpolated into a template, rejecting path-traversal/segment-injection attempts
  (e.g. an id containing `/` or `..`) independently of Core's own malformed-id handling
  (D-0010).
- **Each Next.js route file exports only its allowlisted HTTP methods** (e.g.
  `content-sources/route.ts` exports `GET`/`POST` only, nothing else) — Next.js returns `405`
  for any other verb automatically, so the method allowlist is structural, not a runtime check
  that can be bypassed.
- **The Clerk session token is obtained and forwarded server-side only.** Each route handler
  calls `auth()` (App Router server API) inside `callCore`, reads the current session's
  `getToken()`, and forwards it as `Authorization: Bearer <token>` to Core. The token never
  reaches the browser via this path (it was already there from Clerk's own client SDK, but this
  BFF layer never logs, echoes, or stores it).
- **`SIDUS_CORE_API_URL` is the only Core target, and it is server-only.** It is read once per
  call from `process.env` — never exposed as `NEXT_PUBLIC_*`, never accepted from a request
  header/body/query param. Core itself has no path prefix (routes are mounted at the root, see
  `services/core/main.go`), so the fixed operation path is appended directly to this base URL.
- **Core remains the sole authorization authority.** The BFF performs no role check of its own
  before forwarding a request — it only fails closed on missing configuration/token (below).
  Every mutation is still gated by Core's existing `auth.Protect` (401 missing/invalid token,
  403 valid token lacking the permission — D-0006). The web app's own role check
  (`lib/editorial/permissions.ts`, `lib/editorial/role.ts`) is **UI-visibility only**: it
  decides which controls render, never what Core will accept. A stale or wrong value there
  cannot grant access Core would refuse.
- **Body handling.** Incoming `POST`/`PATCH` bodies are validated (`readSafeJsonBody`:
  `application/json` content type required, ≤100 KB, syntactically valid JSON) and then
  forwarded **verbatim** (the original raw text, never re-serialized) so Core's own strict,
  case-sensitive field allowlist decoding (D-0010) is the real authority on shape. The approve
  route ignores any client body entirely and always sends `{}` — approval carries no
  client-suppliable fields.
- **Responses are passed through, not rewrapped**, except for the BFF's own failure modes
  below — Core's response bodies are already safe/generic per D-0007–D-0010.

## Fail-closed behavior

`callCore` checks, in order, before any network call:

1. `SIDUS_CORE_API_URL` unset/blank → `503 service_unavailable` (mirrors how
   `services/core/main.go` refuses to mount routes without `DATABASE_URL`/Clerk configured).
2. No Clerk session / `getToken()` returns null → `401 unauthorized`.
3. `{id}` fails the id pattern → `400 invalid_id`, no Core call.

A `fetch` failure (network error, DNS, connection refused) maps to a generic
`502 upstream_unavailable`; a non-JSON or unparsable upstream response maps to a generic
`502 upstream_error`. Neither ever forwards the underlying error text, a stack trace, or the
target URL. Requests to Core carry a 10s timeout (`AbortController`). Nothing in this layer
ever logs a bearer token, the Core URL, a source field value, or approval/rejection data —
there is no logging call in `lib/editorial/*` or `app/api/editorial/*` at all.

**Core 5xx and redirects (T-0009 review, D-0011 update):** any Core response with
`status >= 500` is never returned to the browser — its body is read (to release the
connection) and discarded, never logged, and `callCore` throws the same generic
`502 upstream_error` / "the editorial service is temporarily unavailable" regardless of what
Core actually sent. This closes the leak where a raw Go panic or Postgres driver error could
otherwise reach the browser unchanged. Core `401`/`403` and validation/domain `4xx` responses
are still passed through unchanged — only `5xx` is intercepted. The upstream `fetch` call sets
`redirect: "error"`, so a Core redirect is never followed and the bearer token is never
forwarded to a redirect target; it surfaces as the same generic `502 upstream_unavailable`
fetch-failure path above.

## Roles

Mirrors the existing content-source/catalogue role matrix (D-0006, `docs/auth-setup.md`) —
this task added no new role or permission:

| Role | Sees editorial nav / page | Read / create / edit pending sources | Approve / reject |
| --- | --- | --- | --- |
| `learner` | No | No | No |
| unknown/missing | No | No | No |
| `editor` | Yes | Yes | No |
| `reviewer` | Yes | Yes | Yes |
| `admin` | Yes | Yes | Yes |

`app/layout.tsx` reads the caller's role (`lib/editorial/role.ts` →
`getOptionalEditorialRole()`, non-forcing, safe on public routes) to decide whether to render
the "Editorial sources" nav link. `/dashboard/editorial/sources/page.tsx` requires a signed-in
session (`requireEditorialRole()`, `auth.protect()`, same pattern as the existing `/dashboard`
placeholder) and renders `EditorialSourcesScreen`, which shows an "No editorial access" state
and performs **no** API calls for `learner`/unknown roles, or the interactive workspace
otherwise.

## Environment setup

Add to the gitignored `apps/web/.env.local` (never `.env.example`, never committed):

```
SIDUS_CORE_API_URL=http://localhost:8080
```

Core must already be running with `DATABASE_URL` and Clerk configured (see
`docs/auth-setup.md` → "Fail-closed configuration"); otherwise Core's own routes stay
unmounted and every editorial call returns a Core-side `404` proxied through as-is.

## Manual approval/link procedure (human step, outside this task)

This task builds the workflow; it does not perform any approval or link itself (that stays a
human editorial decision per D-0005's rights-safety stance). To progress the two seeded 0610/
5090 sources (T-0001) toward approval:

1. Sign in as a user with `sidus_role` = `editor` (or higher) per `docs/auth-setup.md`.
2. Open `/dashboard/editorial/sources`.
3. Select the pending source, fill in the missing required fields shown in the "Missing for
   approval" column (`owner`, `sourceHash`, `licenceReference`, `permittedUse`,
   `allowedAudience` — `title`/`sourceUrl` are already seeded), and save.
4. If the source's syllabus association needs completing/correcting, pick the syllabus from
   the dropdown and save — this resolves against the curriculum catalogue registry exactly as
   `docs/provenance-catalogue-linking.md` describes (a re-supplied identical code links the
   catalogue syllabus id without re-claiming `syllabusCode` changed).
5. Sign in as (or switch to) a user with `sidus_role` = `reviewer` or `admin`.
6. Open the same source, review the now-complete metadata, and use Approve or Reject (reject
   requires a reason). Both require an explicit inline confirmation step before the request is
   sent.

## No-content policy

This workflow only ever displays and edits **metadata** already defined by D-0005:
title/owner/URL/hash/licence reference/permitted use/allowed audience/syllabus association and
lifecycle status. It never displays, accepts, or stores the source material itself — no PDF,
extracted text, diagram, screenshot, past paper, mark scheme, or question content anywhere in
the UI, BFF, or its tests. Curriculum-map and question/rubric authoring UI are explicitly out
of scope for this task (T-0006/T-0007 remain code-only, unauthored).

## Testing

- `lib/editorial/core-proxy.test.ts`, `lib/editorial/permissions.test.ts`: fail-closed
  behavior, the fixed operation→URL mapping, id-validation, bearer forwarding, Core
  passthrough, generic upstream-failure mapping, no secret logging.
- `app/api/editorial/**/route.test.ts`: each route file exports exactly its intended HTTP
  method(s); correct delegation to `callCore`/`readSafeJsonBody`.
- `app/dashboard/editorial/sources/workspace.test.tsx`,
  `editorial-sources-screen.test.tsx`: loading/empty/error states, list rendering, create/edit
  flows (including the "cannot clear a field" and "non-pending sources aren't editable" rules),
  role-gated review controls, and the reject-requires-reason-and-confirmation flow.

Run: `cd apps/web && npm run test` (Vitest) and `npm run typecheck` / `npm run build`.
