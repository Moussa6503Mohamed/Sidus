# Curriculum map

Metadata-only infrastructure for curriculum-map nodes — topic maps, learning objectives,
practical skills, and assessment rules — scoped under a [curriculum-catalogue](curriculum-catalogue.md)
syllabus. See decision [D-0008](decisions.md).

**No syllabus text, objective wording, topic labels, assessment text, questions, mark schemes,
PDFs, extracts, diagrams, OCR output, or any other derivative content is stored here.** A
node's `label`/`sourceLocator` fields are editorial identity/reference metadata only, authored
through the private, approved [editorial curriculum-map workflow](editorial-curriculum-workflow.md)
(T-0010). **No map data is seeded by this task or by T-0010** — every node is created explicitly
by a human editor through that UI.

## Prerequisite: the source gate

Every node must reference an approved `content_sources` row (the [T-0001 rights
gate](content-provenance-register.md)) whose `catalogue_syllabus_id` matches the node's
syllabus. Before this can happen for any source, a human editor/admin must:

1. Complete and approve the source's rights metadata (`POST /content-sources/{id}/approve`) —
   see [content-provenance-register.md](content-provenance-register.md).
2. Confirm the catalogue link via `PATCH /content-sources/{id}` — see
   [provenance-catalogue-linking.md](provenance-catalogue-linking.md).

Only after both steps does a source pass the curriculum-map gate. Map content itself (topic
labels, objective wording, etc.) is then authored through the private, approved
[editorial curriculum-map workflow](editorial-curriculum-workflow.md) (T-0010) — never as raw
API calls outside that workflow's authorization/audit path.

## Schema

Tables live in `services/core/migrations` (0010–0011).

### `curriculum_map_nodes`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | UUID PK | stable id |
| `syllabus_id` | UUID FK → `syllabuses` | not changeable after creation (see "Why syllabus is immutable" below) |
| `parent_node_id` | UUID FK → `curriculum_map_nodes`, nullable | must belong to the same syllabus; no cycles |
| `node_kind` | TEXT | `topic` \| `objective` \| `practical_skill` \| `assessment_rule` |
| `node_code` | TEXT | stable editorial reference/code, unique within its syllabus |
| `label` | TEXT | editorial label/summary placeholder for the future private workflow |
| `status` | TEXT | `draft` \| `verified` \| `retired` (default `draft`) |
| `content_source_id` | UUID FK → `content_sources`, **NOT NULL** | must pass the source gate on every write that sets it |
| `source_locator` | TEXT, nullable | optional reference metadata (e.g. a section label) pointing at the source — never source content |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

**Uniqueness:** `UNIQUE (syllabus_id, node_code)`.

### `curriculum_map_events` (immutable audit)

Append-only trail: `node_id`, `event_type`
(`node_created`/`node_updated`/`node_verified`/`node_retired`), `actor_id` (the **verified
Clerk subject**), `event_time`, `changed_fields` (names only — never values). A `BEFORE UPDATE
OR DELETE` trigger rejects any mutation, mirroring `syllabus_events`/`content_source_events`.

## Safe constraints

- **Node parent must belong to the same syllabus.** Enforced at the application layer, inside
  the same transaction as the write, with a row lock on the candidate parent (`SELECT ...
  FOR UPDATE`) — not a DB trigger. This keeps `invalid_parent` error mapping under Core's
  control instead of parsing a trigger-raised message (see D-0008 "Alternatives").
- **No cycles in the hierarchy.** Also application-layer: on any write that changes
  `parentNodeId`, Core walks the candidate parent's ancestor chain (bounded by
  `maxParentHops`, same transaction) and rejects if it reaches the node being written.
  **Every ancestor row traversed is locked `FOR UPDATE`**, not only the candidate parent — an
  unlocked walk could read a chain another transaction is re-parenting and admit a cycle. The
  locks are held until the writing transaction ends. Consequence: two transactions re-parenting
  overlapping chains in opposite orders can deadlock; Postgres aborts one, which surfaces as a
  generic `500 internal_error` with **no partial write**, and the caller may retry.
- **No duplicate `nodeCode` within a syllabus.** DB unique index; a collision maps to `409
  duplicate_node_code`.
- **Source FK required.** `content_source_id` is `NOT NULL`.
- **The source gate re-runs on every node write.** Not only on create and not only when
  `contentSourceId` changes: `PATCH`, `verify`, and `retire` all re-validate the node's
  *effective* source (the supplied one if the request carries `contentSourceId`, otherwise the
  stored one) against the node's immutable syllabus, before any node row, status change, or
  audit event is written. A source that was approved and linked when the node was created can
  later be un-approved, unlinked, or re-linked to a different syllabus; without this
  re-validation a node could keep being edited and even promoted to `verified` while grounded
  in a source that no longer passes the T-0001 rights gate.
  - A failed gate is a stable `400` (`unknown_source` / `unapproved_source` /
    `unlinked_source` / `mismatched_source`) and causes **no** node mutation, **no** status
    transition, **no** audit event, and **no** `updated_at` change.
  - **Operational consequence:** while a node's source is in a failed state the node is frozen
    — it cannot be edited, verified, *or retired*. Restore the source (re-approve and/or
    re-link it via the content-source API) to unfreeze the node, then retire it if that was the
    intent.
- **The gate is Core-only.** No other service — in particular the AI service — may create,
  update, verify, or retire a node, or evaluate the gate.

### Reads return every lifecycle status (T-0010)

`GET /curriculum-map/nodes` and `GET /curriculum-map/nodes/{id}` return nodes of **any**
lifecycle status (draft, verified, retired) to a `curriculum_map:read` holder — this permission
is held only by `editor`/`reviewer`/`admin` (see "Roles" below); there is no learner-facing or
public consumer of this route. The original verified-only reader rule assumed a future public
reader would share this route; the T-0010 editorial UI showed that assumption blocked the
workflow instead — an editor could not re-open their own draft, and a reviewer had no way to
discover a draft to verify or retire. See D-0008 "Update (T-0010)". The list endpoint accepts an
optional `status` query parameter (`draft` | `verified` | `retired` | `all`) to narrow results;
omitting it or supplying `all` returns every status. An unrecognized value is `400
invalid_status`. No write-side
rule (source gate, parent/cycle checks, lifecycle transitions) changed.

### Why syllabus is immutable on a node

`syllabusId` is set at creation and cannot be changed by `PATCH`. Changing it would require
re-validating an entire subtree's parent-same-syllabus invariant in one write; creating a new
node under the correct syllabus is simpler and does not risk a partially-migrated hierarchy.

## Lifecycle

```
draft --(verify)--> verified --(retire)--> retired
draft --(retire)-----------------------------^
```

- **Create** always produces a `draft` node (status is never caller-settable at creation).
- **PATCH** (`parentNodeId`, `nodeKind`, `nodeCode`, `label`, `contentSourceId`,
  `sourceLocator` — and nothing else) only succeeds while `status = draft`; otherwise `409
  invalid_lifecycle_transition`. `status` itself is never PATCHable — only `verify`/`retire`
  change it.
- **Verify** (`draft → verified`) requires `curriculum_map:verify`.
- **Retire** (`draft → retired` or `verified → retired`) requires `curriculum_map:verify`.
  Retiring an already-retired node is `409 invalid_lifecycle_transition`.
- Every one of `PATCH`/`verify`/`retire` also re-runs the source gate (see "Safe constraints").
- Reader endpoints (list/get) return nodes of any lifecycle status to a `curriculum_map:read`
  holder (T-0010; see "Reads return every lifecycle status" above) — draft/retired nodes are
  not hidden from editorial staff, only from a hypothetical public consumer that does not exist.

## API

All endpoints require a Clerk session bearer token (see [auth-setup.md](auth-setup.md)). The
audit actor is always the verified Clerk subject — request bodies carry no actor field.

### Strict request decoding

Both `POST` and `PATCH` accept exactly one JSON object and reject anything else with a stable
`400 invalid_json` (the message never echoes the offending field back):

- **Unknown fields are rejected**, including identity fields (`actorId`, `reviewerId`),
  immutable fields (`syllabusId`, `id`, `createdAt`, `updatedAt`), the lifecycle field
  (`status`), field-name typos, and case variants of a real field (`Label`).
- **Trailing JSON values or junk after the object are rejected** (`{...}{}`, `{...}[1]`,
  `{...}not-json`).
- A rejected request is refused before any store call: no node mutation, no status transition,
  no audit event.
- `PATCH` decodes into a `map[string]json.RawMessage` so it can distinguish "field absent" from
  "field present as `null`" for the two nullable fields. `json.Decoder.DisallowUnknownFields`
  has **no effect on a map destination**, so `PATCH` enforces an explicit allowlist of the six
  updatable field names (`updatablePatchFields` in `handlers.go`) in addition to a strict
  struct decode. Only `parentNodeId` and `sourceLocator` may be explicitly `null`, which clears
  them; an absent field is left unchanged.

| Method & path | Permission | Roles | Notes |
| --- | --- | --- | --- |
| `GET /curriculum-map/nodes?syllabusId=...&status=...` | `curriculum_map:read` | editor, reviewer, admin | any lifecycle status when `status` is omitted or `all`; `draft`/`verified`/`retired` narrow results; `syllabusId` required, must resolve to an **active** catalogue syllabus |
| `GET /curriculum-map/nodes/{id}` | `curriculum_map:read` | editor, reviewer, admin | any lifecycle status (404 only if the node does not exist) |
| `POST /curriculum-map/nodes` | `curriculum_map:create` | editor, reviewer, admin | creates a draft |
| `PATCH /curriculum-map/nodes/{id}` | `curriculum_map:create` | editor, reviewer, admin | draft only |
| `POST /curriculum-map/nodes/{id}/verify` | `curriculum_map:verify` | reviewer, admin | draft → verified |
| `POST /curriculum-map/nodes/{id}/retire` | `curriculum_map:verify` | reviewer, admin | draft/verified → retired |

`401` missing/invalid token; `403` valid token lacking permission; `400` validation
(`missing_required_fields`, `invalid_node_kind`, `invalid_status`, `blank_field(s)`,
`no_updatable_fields`, `no_changes`, `unknown_syllabus`, `invalid_parent`, `unknown_source`,
`unapproved_source`, `unlinked_source`, `mismatched_source`, `invalid_json`); `409` conflict
(`duplicate_node_code`, `invalid_lifecycle_transition`); `404` not found.

`500 internal_error` (database, scan, transaction, or other infrastructure failure) always
returns the same stable, generic message. Raw driver/Go error text is never forwarded to a
client.

### Listing is validated, not silently empty

`GET /curriculum-map/nodes` resolves `syllabusId` against the catalogue **before** returning a
result:

- unknown id, malformed (non-UUID) id, or a `draft`/`retired` syllabus → `400 unknown_syllabus`;
- `status=all` or omitted → no lifecycle SQL filter; an unrecognized `status` value → `400 invalid_status`;
- known **active** syllabus with no matching nodes → `200` with an empty `items` list.

A typo'd or retired syllabus is therefore never indistinguishable from a real syllabus whose
map has not been authored yet — which matters while the map is empty by design.

## Roles

Least privilege (see `services/core/internal/auth`). Learner and unknown roles have **no**
curriculum-map access.

| Role | read (verified) | create/update draft | verify / retire |
| --- | --- | --- | --- |
| learner | — | — | — |
| editor | ✓ | ✓ | — |
| reviewer | ✓ | ✓ | ✓ |
| admin | ✓ | ✓ | ✓ |

## Onboarding sequence — from an approved source to a verified node

No code change is required for a new syllabus's map content. The sequence:

1. **Source rights approval** — a human completes and approves the content source's rights
   metadata (T-0001): `POST /content-sources/{id}/approve`.
2. **Catalogue link confirmation** — a human confirms the source's `catalogue_syllabus_id`
   (T-0005): `PATCH /content-sources/{id}` with the syllabus code.
3. **Author map content** — an editor/reviewer/admin uses the private
   [editorial curriculum-map workflow](editorial-curriculum-workflow.md) (T-0010) to author the
   actual topic/objective/practical-skill/assessment-rule editorial label/code metadata (never
   copied syllabus text).
4. **Create draft node** — an editor/reviewer/admin calls `POST /curriculum-map/nodes` with the
   syllabus id, node kind, a stable per-syllabus `nodeCode`, `label`, and the approved+linked
   `contentSourceId`. The source gate runs before any write.
5. **Verify** — a reviewer/admin calls `POST /curriculum-map/nodes/{id}/verify` once the draft
   is confirmed correct.
6. **Retire** (as needed) — a reviewer/admin calls `POST /curriculum-map/nodes/{id}/retire` to
   withdraw a node while keeping its history in `curriculum_map_events`.

## What consumes a verified node

Original questions (T-0007) are grounded in exactly one **verified** node: a question stores a
required `curriculum_map_node_id`, and every question write re-validates that the node is still
verified, still belongs to the question's syllabus, and that its content source still passes the
source gate above. Retiring a node therefore freezes the questions grounded in it. See
[question-rubric-model.md](question-rubric-model.md).

**No data is seeded.** The seeded 0610 source remains `pending`/unlinked and no 9700 source exists.
Until a human completes steps 1–2 above, no curriculum-map node can pass the source gate for
either active Biology syllabus. Historical 5090 is retired.
