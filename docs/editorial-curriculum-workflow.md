# Editorial curriculum-map workflow (T-0010)

The second secure web-to-Core workflow for editorial staff, alongside
[editorial-source-workflow.md](editorial-source-workflow.md) (T-0009): author, edit, review,
verify, and retire [curriculum-map](curriculum-map.md) nodes for an active catalogue syllabus.
It is a UI/BFF surface over the existing T-0006 curriculum-map API plus one tiny, essential
Core read-contract widening — see [D-0008 "Update (T-0010)"](decisions.md) — no schema,
migration, write-side business rule, or seed content was added. This task performs no
automatic creation, approval, link, verification, or retirement of node or source data itself;
every action is an explicit human click through the new UI.

## Architecture

```
Browser  →  Next.js route handlers (app/api/editorial/curriculum-map/*)  →  Sidus Core
(fetch)     (same-origin, server-only)                                      (sole authorization authority)
```

This reuses the T-0009/[D-0011](decisions.md) BFF architecture unchanged:

- **The browser never talks to Core directly.** `/dashboard/editorial/curriculum` and its
  client components call only same-origin `app/api/editorial/curriculum-map/*` route handlers.
- **`lib/editorial/core-proxy.ts`'s closed `EditorialOperation` union gained six variants** —
  `listCurriculumMapNodes`, `getCurriculumMapNode`, `createCurriculumMapNode`,
  `updateCurriculumMapNode`, `verifyCurriculumMapNode`, `retireCurriculumMapNode` — each mapped
  to exactly one fixed Core method+path template. `{id}` is validated the same way as every
  other operation (`^[A-Za-z0-9_-]{1,128}$`) before interpolation; there is no caller-controlled
  target URL.
- **Each Next.js route file exports only its allowlisted HTTP method(s)** — unsupported verbs
  get Next's structural `405`.
- **`callCore`, `readSafeJsonBody`, the Clerk bearer-token forwarding, `SIDUS_CORE_API_URL`
  sourcing, fail-closed ordering, Core-5xx sanitization, and redirect refusal are all shared,
  unmodified code** from T-0009 — see `docs/editorial-source-workflow.md` for the full behavior.
  No new environment variable is needed.
- **Verify and retire ignore any client body and always send `{}`** — same pattern as the
  existing approve route (they carry no client-suppliable fields).
- **Core remains the sole authorization authority.** The web role check
  (`lib/editorial/permissions.ts`/`role.ts`, unchanged from T-0009) is UI-visibility only.

## Reads return every lifecycle status (Core contract change)

Building this UI is what exposed a gap in the original T-0006 design: `GET /curriculum-map/
nodes` and `GET /curriculum-map/nodes/{id}` had returned **verified nodes only**, but
`curriculum_map:read` is held only by `editor`/`reviewer`/`admin` — there is no learner-facing
or public consumer of that route. Verified-only reads meant an editor could never re-open their
own draft to keep editing it, and a reviewer had no way to discover a draft to verify or retire
(a node's id was otherwise visible only in the create/update response body from the same
request that produced it). Core now returns nodes of **any** lifecycle status to a
`curriculum_map:read` holder; the list endpoint accepts an optional `status` query parameter
(`draft` | `verified` | `retired`) to narrow results, defaulting to every status. No write-side
rule (source gate, parent/cycle checks, lifecycle transitions) changed. See
[D-0008 "Update (T-0010)"](decisions.md) and `docs/curriculum-map.md`.

## Roles

Mirrors the curriculum-map permission matrix (D-0008) — this task added no new role or
permission:

| Role | Sees curriculum nav / page | List / create / edit draft nodes | Verify / retire |
| --- | --- | --- | --- |
| `learner` | No | No | No |
| unknown/missing | No | No | No |
| `editor` | Yes | Yes | No |
| `reviewer` | Yes | Yes | Yes |
| `admin` | Yes | Yes | Yes |

`app/layout.tsx`'s existing role-gated nav block gained a second link, "Curriculum map",
alongside "Editorial sources", both driven by the same `isEditorialRole(role)` check.
`/dashboard/editorial/curriculum/page.tsx` requires a signed-in session
(`requireEditorialRole()`) and renders `CurriculumMapScreen`, which shows `AccessDenied` (zero
API calls) for `learner`/unknown, or the interactive workspace otherwise.

## Workspace UI

- **Syllabus selector** — scopes the workspace to one active catalogue syllabus at a time (also
  the source of `syllabusId` for a new node; syllabus is immutable on a node after creation, per
  `docs/curriculum-map.md` → "Why syllabus is immutable on a node").
- **Node list** — table of the selected syllabus's nodes: code, kind, label, parent (resolved to
  the parent's `nodeCode`, with a computed display-only indent showing hierarchy depth — purely
  presentational; Core's parent/cycle validation is the real authority), lifecycle status badge,
  and the linked content source's title.
- **Node form** (create/edit) — node kind, node code, editorial label, optional parent node
  (any node in the same syllabus regardless of status), an **approved** content source filtered
  to sources already linked to the selected syllabus (with a clear message when none exist yet
  — link one via `/dashboard/editorial/sources` first), and an optional source locator. Editable
  only while a node is `draft`, mirroring the source-workflow's pending-only edit rule.
  `parentNodeId`/`sourceLocator` support an explicit clear (Core's `null`-clears-on-PATCH
  semantics); `nodeCode`/`label`/`contentSourceId` cannot be cleared, only changed.
- **Review actions** (reviewer/admin only) — `Verify` (draft → verified) and `Retire` (draft or
  verified → retired), each behind an explicit inline confirmation step, matching the
  approve/reject confirmation pattern from the source workflow. An already-retired node shows no
  action.
- **States** — loading, empty ("no nodes yet"), error banner with retry, create, edit, verify
  confirmation, retire confirmation, disabled (non-draft) editing, and a mobile-safe responsive
  layout reusing `app/dashboard/editorial/sources/styles.module.css`'s existing design language
  (buttons, form, table, banners) via CSS Modules `composes`, with a small local stylesheet
  adding only the new pieces (lifecycle status-badge color variants, the syllabus picker, and
  hierarchy indentation).

## No-content policy

Same as the source workflow: this UI only ever displays and edits **metadata** — node kind,
code, editorial label, parent linkage, content-source linkage, source locator, and lifecycle
status. It never displays, accepts, or stores syllabus text, objective wording, topic labels,
assessment text, questions, mark schemes, PDFs, extracts, diagrams, screenshots, past papers, or
any other derivative content anywhere in the UI, BFF, or its tests.

## Testing

- `lib/editorial/core-proxy.test.ts`: the six new operation→URL mappings, alongside the
  existing content-source/syllabus coverage.
- `app/api/editorial/curriculum-map/nodes/**/route.test.ts`: each route file exports exactly
  its intended HTTP method(s); correct delegation to `callCore`/`readSafeJsonBody`; verify/retire
  ignore the client body and always send `{}`.
- `app/dashboard/editorial/curriculum/workspace.test.tsx`,
  `curriculum-screen.test.tsx`: loading/empty/error states, hierarchy/status/source rendering,
  create/edit flows (including the "cannot clear a required field" and "non-draft nodes aren't
  editable" rules), role-gated review controls, verify/retire confirmation flows, and the
  access-denied zero-API-call state.
- `services/core/internal/curriculummap`: existing handler/store tests updated for the widened
  read contract, plus new coverage for the `status` filter and its validation
  (`invalid_status`).

Run: `cd apps/web && npm run test` (Vitest), `npm run typecheck` / `npm run build`; Go tests via
`docs/local-setup.md`'s Docker commands.
