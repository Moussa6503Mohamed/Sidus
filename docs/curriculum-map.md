# Curriculum map

Metadata-only infrastructure for curriculum-map nodes — topic maps, learning objectives,
practical skills, and assessment rules — scoped under a [curriculum-catalogue](curriculum-catalogue.md)
syllabus. See decision [D-0008](decisions.md).

**No syllabus text, objective wording, topic labels, assessment text, questions, mark schemes,
PDFs, extracts, diagrams, OCR output, or any other derivative content is stored here.** A
node's `label`/`sourceLocator` fields are editorial identity/reference metadata only — a
placeholder for a future **private, approved** authoring workflow. **No map data is seeded by
this task.**

## Prerequisite: the source gate

Every node must reference an approved `content_sources` row (the [T-0001 rights
gate](content-provenance-register.md)) whose `catalogue_syllabus_id` matches the node's
syllabus. Before this can happen for the seeded 0610/5090 sources, a human editor/admin must:

1. Complete and approve the source's rights metadata (`POST /content-sources/{id}/approve`) —
   see [content-provenance-register.md](content-provenance-register.md).
2. Confirm the catalogue link via `PATCH /content-sources/{id}` — see
   [provenance-catalogue-linking.md](provenance-catalogue-linking.md).

Only after both steps does a source pass the curriculum-map gate. Map content itself (topic
labels, objective wording, etc.) is then authored through a **future private approved
workflow**, not this API — this task ships infrastructure only.

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
  `parentNodeId`, Core walks the candidate parent's ancestor chain (bounded, same transaction,
  same row locks) and rejects if it reaches the node being written.
- **No duplicate `nodeCode` within a syllabus.** DB unique index; a collision maps to `409
  duplicate_node_code`.
- **Source FK required.** `content_source_id` is `NOT NULL`; every create, and every update
  that changes it, re-runs the source gate.
- **Source cannot be changed to a mismatched syllabus.** The gate re-runs against the node's
  (immutable) syllabus on every `contentSourceId` change.

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
  `sourceLocator`) only succeeds while `status = draft`; otherwise `409
  invalid_lifecycle_transition`. `status` itself is never PATCHable — only `verify`/`retire`
  change it.
- **Verify** (`draft → verified`) requires `curriculum_map:verify`.
- **Retire** (`draft → retired` or `verified → retired`) requires `curriculum_map:verify`.
  Retiring an already-retired node is `409 invalid_lifecycle_transition`.
- Reader endpoints (list/get) only ever return **verified** nodes — mirrors the catalogue's
  active-only reader pattern.

## API

All endpoints require a Clerk session bearer token (see [auth-setup.md](auth-setup.md)). The
audit actor is always the verified Clerk subject — request bodies carry no actor field (strict
decoder rejects unknown fields and trailing JSON).

| Method & path | Permission | Roles | Notes |
| --- | --- | --- | --- |
| `GET /curriculum-map/nodes?syllabusId=...` | `curriculum_map:read` | editor, reviewer, admin | verified nodes only; `syllabusId` required |
| `GET /curriculum-map/nodes/{id}` | `curriculum_map:read` | editor, reviewer, admin | verified only (404 otherwise) |
| `POST /curriculum-map/nodes` | `curriculum_map:create` | editor, reviewer, admin | creates a draft |
| `PATCH /curriculum-map/nodes/{id}` | `curriculum_map:create` | editor, reviewer, admin | draft only |
| `POST /curriculum-map/nodes/{id}/verify` | `curriculum_map:verify` | reviewer, admin | draft → verified |
| `POST /curriculum-map/nodes/{id}/retire` | `curriculum_map:verify` | reviewer, admin | draft/verified → retired |

`401` missing/invalid token; `403` valid token lacking permission; `400` validation
(`missing_required_fields`, `invalid_node_kind`, `blank_field(s)`, `no_updatable_fields`,
`no_changes`, `unknown_syllabus`, `invalid_parent`, `unknown_source`, `unapproved_source`,
`unlinked_source`, `mismatched_source`, `invalid_json`); `409` conflict
(`duplicate_node_code`, `invalid_lifecycle_transition`); `404` not found.

`500 internal_error` (database, scan, transaction, or other infrastructure failure) always
returns the same stable, generic message. Raw driver/Go error text is never forwarded to a
client.

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
3. **Author map content (future, out of scope here)** — a private, approved editorial workflow
   produces the actual topic/objective/practical-skill/assessment-rule text; this task does not
   provide that workflow, only the infrastructure it will write into.
4. **Create draft node** — an editor/reviewer/admin calls `POST /curriculum-map/nodes` with the
   syllabus id, node kind, a stable per-syllabus `nodeCode`, `label`, and the approved+linked
   `contentSourceId`. The source gate runs before any write.
5. **Verify** — a reviewer/admin calls `POST /curriculum-map/nodes/{id}/verify` once the draft
   is confirmed correct. Only verified nodes are visible to readers.
6. **Retire** (as needed) — a reviewer/admin calls `POST /curriculum-map/nodes/{id}/retire` to
   withdraw a node while keeping its history in `curriculum_map_events`.

**No data is seeded.** The two pre-existing 0610/5090 content sources remain `pending`/unlinked
until a human completes steps 1–2 above; until then, no curriculum-map node can pass the source
gate for either syllabus.
