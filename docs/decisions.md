# Decision log

## D-0001 — Platform split

**Status:** Approved
**Decision:** Next.js/TypeScript frontend; Go core; Python/FastAPI AI services.
**Reason:** Web/PWA speed, high-traffic core efficiency, Python AI/OCR ecosystem.

## D-0002 — Storage direction

**Status:** Approved
**Decision:** PostgreSQL system of record; Redis, object storage, OpenSearch later.
**Reason:** Strong consistency and auditability now; scale services later.

## D-0003 — AI policy

**Status:** Approved
**Decision:** Anthropic only. Haiku routine work; Sonnet complex marking. Verified explanation cache blocks identical regeneration.
**Reason:** Cost control and predictable quality.

## D-0004 — First vertical slice

**Status:** Superseded by D-0013
**Decision:** Cambridge IGCSE Biology 0610 Extended and Cambridge O Level Biology 5090.
**Reason:** Defined initial learning scope.

## D-0005 — Copyright and source handling

**Status:** Approved
**Decision:** Public repository stores metadata, code, original content, and approved assets only. No copyrighted source PDFs, extracted text, diagrams, or derivative questions.
**Reason:** Rights safety.

## D-0006 — Authentication and authorization

**Status:** Approved
**Decision:** Clerk owns authentication (issues/signs session JWTs); Sidus Core owns
authorization. Backends verify the Clerk session JWT offline via JWKS (Core: official
`clerk-sdk-go/v2`; AI: PyJWT `PyJWKClient`), validating signature, expiry, issuer, and
authorized party, with JWKS/keys cached (no Clerk Backend API call per request). The audit
actor and review reviewer are the verified session `sub` only — `actorId`/`reviewerId` are
removed from all request bodies. Roles come from the verified `sidus_role` claim
(`learner` < `editor` < `reviewer` < `admin`); missing/unknown role is denied by default.
`401` for missing/invalid token, `403` for valid token lacking permission. Content-source
routes fail closed: they mount only when both the database and Clerk are configured.
**Reason:** No custom password handling; identity cannot be spoofed via request bodies;
least-privilege access to the rights/provenance surface; cost control (no per-request
Backend API calls).
**Alternatives:** Hand-rolled JWT/JWKS verification (rejected: reinvents the SDK, more
audit surface); calling the Clerk Backend API per request (rejected: latency/cost);
trusting body-supplied actor identity (rejected: spoofable, breaks audit integrity).
**Owner/date:** Claude Code agent, 2026-07-24 (T-0003).

## D-0007 — Curriculum catalogue authority

**Status:** Approved
**Decision:** Sidus Core owns a metadata-only curriculum catalogue: normalized `subjects` and
`syllabuses` tables (plus an immutable `syllabus_events` audit trail) are the single authority
for which syllabuses exist. A syllabus record carries board, syllabus code, subject relation,
qualification/level, optional track (e.g. Extended), display name, optional curriculum
year/edition (stored only when explicitly known — never inferred), lifecycle status
(`draft`/`active`/`retired`), and timestamps. Safe uniqueness is `(board, syllabus_code,
COALESCE(track,''))` — a syllabus code is never assumed globally unique, and different boards
may reuse a code. Content sources gain a nullable FK (`catalogue_syllabus_id`) to the
catalogue, added non-destructively; the hard-coded `0610`/`5090` request/enum validation is
replaced by registry-backed validation: a supplied code must resolve to exactly one **active**
catalogue syllabus (unknown/inactive/ambiguous → stable `400` before any DB write), while an
omitted code stays allowed for pending source metadata. Catalogue reads (list/get **active**
syllabuses and subjects) require `content_catalogue:read` (editor/reviewer/admin); create/change
require `content_catalogue:manage` (admin only). Learner and unknown roles are denied. Catalogue
mutations are audited with the verified Clerk subject and names-only (non-content) changed-field
lists — including subject creation, via an immutable `subject_events` trail (migration 0009)
written in the same transaction as the subject row, so a failed audit insert rolls back the
subject. The seeded Biology subject (migration 0007) is bootstrap data inserted outside the
application path and intentionally carries no `subject_events` row (no invented actor id).
Catalogue HTTP endpoints never return raw database/scan/transaction error text: infrastructure
failures map to one stable `internal_error` message; only static, non-sensitive domain errors
(duplicate/unknown/not-found/invalid-status/no-changes) carry descriptive text. Only the two
D-0004 biology syllabuses are seeded (`active`); no curriculum is inferred from the copyrighted
inventory, and no source material is stored. This is the all-subject beta path: future approved
Cambridge syllabuses are onboarded as data (an admin API call), not code.
**Reason:** All-subject beta needs syllabus onboarding without code changes; the copyright gate
(D-0005) forbids inferring curricula or storing source material; least-privilege access and an
immutable audit trail protect the catalogue surface; safe uniqueness avoids collapsing distinct
syllabuses or trusting a non-unique code.
**Alternatives:** Keep the two-code union and add codes by editing code (rejected: does not
scale to all subjects, contradicts beta goal); make `syllabus_code` globally unique (rejected:
codes are only unique per board, and track distinguishes offerings); auto-map the existing
seeded 0610/5090 content_sources rows to catalogue syllabuses in the migration (rejected:
silent mapping of ambiguous rights records — a human links them later); let the AI service hold
catalogue authority (rejected: Core is the single authority; AI adds no content ingestion).
**Owner/date:** Claude Code agent, 2026-07-24 (T-0004).

**Update (T-0005, 2026-08-04):** the migration path above intentionally left every existing
`content_sources` row's `catalogue_syllabus_id` `NULL` rather than auto-mapping it, but the
original `PATCH /content-sources/{id}` `syllabusCode` diff logic only compared the free-text
code — re-supplying an already-stored code was always `400 no_changes`, so there was no safe,
authenticated, audited path to complete that link. `PATCH` now splits the diff: a supplied code
that differs from the stored text still updates `syllabus_code` **and**
`catalogue_syllabus_id` together (audited `changed_fields: ["syllabusCode"]`); a supplied code
that matches the stored text but whose resolved catalogue syllabus differs from the stored FK
(including a `NULL` FK) updates `catalogue_syllabus_id` alone as a link-only human provenance
confirmation (audited `changed_fields: ["catalogueSyllabusId"]` — never claiming `syllabusCode`
changed when it did not); a supplied code matching both stays `400 no_changes`. The request body
is unchanged (still only `syllabusCode`, resolved server-side); no new endpoint, no caller-
supplied catalogue ID, no auto-linking outside an explicit authenticated `PATCH` from an
editor/reviewer/admin. See [provenance-catalogue-linking.md](provenance-catalogue-linking.md).

## D-0008 — Curriculum-map authority and source gate

**Status:** Approved
**Decision:** Sidus Core owns a metadata-only curriculum map: normalized `curriculum_map_nodes`
(plus an immutable `curriculum_map_events` audit trail) hold topic/objective/practical-skill/
assessment-rule structure for a syllabus, scoped under the existing curriculum-catalogue
`syllabuses` registry (D-0007). A node carries a stable id, syllabus FK, optional parent-node FK
(same syllabus only), a `nodeKind` (`topic`/`objective`/`practical_skill`/`assessment_rule`), a
stable per-syllabus `nodeCode`, an editorial `label`/summary placeholder for a future private
approved authoring workflow, a lifecycle `status` (`draft`/`verified`/`retired`), a required
approved `content_sources` FK, an optional `sourceLocator` reference string, and timestamps. The
node table never stores syllabus text, objective wording, topic labels, assessment text,
questions, mark schemes, or any other derivative content — `label`/`sourceLocator` are
identity/reference metadata only, populated later by a private approved workflow outside this
task's scope. Every node write (create, and any update that changes `contentSourceId`) is gated
server-side by Core — the sole authority — verifying the referenced content source exists, is
`approved`, and its `catalogue_syllabus_id` matches the node's syllabus; unknown/unapproved/
unlinked/mismatched sources are rejected with a stable `400` before any write. Parent-same-
syllabus and no-cycle-in-hierarchy are enforced at the application layer inside the same
transaction as the write (row-locked ancestor walk), not by a DB trigger, so Core controls the
exact `invalid_parent` error rather than parsing trigger-raised text; uniqueness of `nodeCode`
per syllabus and the required source FK are DB constraints. `curriculum_map_events` mirrors
`syllabus_events`/`content_source_events`: append-only, `BEFORE UPDATE OR DELETE` trigger,
verified-Clerk-subject actor, changed-field-names only, no values. Least-privilege permissions
`curriculum_map:read` (editor/reviewer/admin — verified nodes only), `curriculum_map:create`
(editor/reviewer/admin — draft create/PATCH), `curriculum_map:verify` (reviewer/admin —
verify/retire) are added to the existing role matrix; learner and unknown roles are denied. No
map data is seeded: a human must first link/approve a content source (T-0001/T-0005), then
author map content through a future private approved workflow.
**Reason:** Topic maps/objectives/practical skills/assessment rules must be buildable without
ever letting copyrighted syllabus/question text enter the repository (D-0005) or letting an
unapproved/mismatched source ground a node (extends the T-0001 rights gate to a new content
type); a single, explicit server-side gate — not a bare FK — is required because "approved" and
"syllabus-matched" are point-in-time facts about another table a FK cannot express; least-
privilege split (editor drafts, reviewer verifies) mirrors the existing content-source
create/review split and lets an editorial pipeline exist before any public-facing map read
surface.
**Alternatives:** Enforce the source gate as a DB trigger (rejected: raw trigger-exception text
would either leak or force fragile message-parsing to map to stable error codes, unlike the
existing catalogue/content-source pattern of Go-layer validation before the write); allow the
AI service to author/verify map nodes (rejected: Core remains the single content-authority, per
D-0007's precedent); make `syllabusId` patchable on an existing node (rejected: would require
re-validating an entire subtree's parent-syllabus consistency on every syllabus change — a new
node under the correct syllabus is simpler and safer); seed placeholder topic/objective data for
the D-0004 biology slice (rejected: no source has completed rights approval yet, so any seeded
node would violate the source gate this task exists to enforce).
**Owner/date:** Claude Code agent, 2026-08-04 (T-0006).

**Update (T-0006 review, 2026-08-04):** four review findings tightened the decision above; the
authority model, schema purpose, role matrix, approval rules, and "no map data seeded" stance
are unchanged.

1. **The source gate now runs on every node write, not only writes that change
   `contentSourceId`.** The original wording ("create, and any update that changes
   `contentSourceId`") left a hole: a source approved and linked at creation can later be
   un-approved, unlinked, or re-linked to a different syllabus, after which a node could still
   be `PATCH`ed and — worse — promoted to `verified` while grounded in a source that no longer
   passes the T-0001 rights gate. `UpdateNode` now re-validates the *effective* source (the
   supplied one if present, otherwise the stored one) against the node's immutable syllabus,
   and `VerifyNode`/`RetireNode` re-validate the stored source, in all cases before any node
   row, status change, or event is written. The accepted consequence is that a node whose
   source has regressed is frozen — including for retirement — until the source is restored;
   freezing was preferred over letting a lifecycle write proceed against a source outside the
   rights gate, since the gate is the reason this table exists. The gate remains Core-only; no
   AI-service authority was added.
2. **Strict JSON is now actually enforced on `PATCH`.** The `PATCH` decoder reads into a
   `map[string]json.RawMessage` to distinguish "absent" from "present as null" for the two
   nullable fields, and `json.Decoder.DisallowUnknownFields` has no effect on a map
   destination — so unknown fields, including `actorId`, `reviewerId`, `syllabusId`, and
   `status`, were silently ignored rather than rejected. This contradicted D-0006 (identity is
   never body-supplied) in spirit even though the ignored values were never used. `PATCH` now
   validates every key against an explicit allowlist of the six updatable field names *and*
   re-decodes strictly into the request struct; unknown/identity/immutable/lifecycle fields,
   typos, case variants, and trailing JSON all return the same stable `400 invalid_json` before
   any store call. Explicit `null` for `parentNodeId`/`sourceLocator` still clears them.
3. **`GET /curriculum-map/nodes` validates the syllabus.** It previously returned `200` with an
   empty list for an unknown or inactive `syllabusId`, making a typo indistinguishable from a
   real but unauthored map — a real risk while no map data is seeded. It now returns `400
   unknown_syllabus` for unknown/malformed/inactive ids, and `200` with an empty list only for
   a known active syllabus with no verified nodes.
4. **The ancestor walk really row-locks the chain.** D-0008 claimed a "row-locked ancestor
   walk", but only the candidate parent was locked `FOR UPDATE`; the walk itself used plain
   `SELECT`s, so a concurrent re-parent could be interleaved and a cycle admitted. Every
   traversed ancestor is now locked `FOR UPDATE` in the writing transaction. The accepted
   trade-off is that two transactions re-parenting overlapping chains in opposite orders can
   deadlock; Postgres aborts one, which surfaces as a generic `500 internal_error` with no
   partial write, and the caller may retry. Bounded traversal (`maxParentHops`) and
   `invalid_parent` mapping are unchanged.

A malformed (non-UUID) syllabus, source, or parent id now maps to the same stable domain error
as a missing one (`unknown_syllabus` / `unknown_source` / `invalid_parent`) instead of a
generic `500`; it is a client error, not an infrastructure failure.

**Update (T-0010, 2026-08-05):** `GET /curriculum-map/nodes` and `GET /curriculum-map/nodes/{id}`
no longer restrict reads to verified nodes. The original "readers only ever see verified nodes"
rule (above, and reaffirmed in the T-0006 review) assumed a future non-editorial reader would
share this route; building the T-0010 editorial UI on top of it exposed that no such reader
exists — `curriculum_map:read` is held only by `editor`/`reviewer`/`admin` — so verified-only
reads instead blocked the workflow this route exists to serve: an editor could never re-open
their own draft to keep editing it, and a reviewer had no way to discover a draft to verify or
retire, since a node's id is otherwise only visible in the create/update response body from the
same request that produced it. `GET /curriculum-map/nodes/{id}` now returns a node of any
lifecycle status to a `curriculum_map:read` holder. `GET /curriculum-map/nodes` gained an
optional `status` query parameter (`draft` | `verified` | `retired` | `all`); omitting it or
supplying `all` returns nodes of every status (the default), and an unrecognized value is a stable `400 invalid_status`
before any store call. No schema, migration, permission, or write-side rule (source gate,
parent/cycle checks, lifecycle transitions) changed — only the read filter. `Store.GetNode`
dropped its `verifiedOnly bool` parameter (there is exactly one caller behavior now); `Store.
ListNodesBySyllabus` replaced `verifiedOnly bool` with `status *Status` (`nil` = all statuses)
to express the three-way filter. The two existing tests that asserted verified-only reads
(`TestGetNode_DraftHiddenFromReaders`, `TestListNodes_ActiveSyllabusWithNoVerifiedNodes_
Returns200Empty`) were rewritten to assert the new contract; new tests cover the `status` filter
and its validation. See `docs/editorial-curriculum-workflow.md`.
**Owner/date:** Claude Code agent, 2026-08-05 (T-0010).

## D-0009 — Original question and versioned rubric authority

**Status:** Approved
**Decision:** Sidus Core owns private, original-question and versioned-rubric infrastructure for a
future Exam Mode: `questions`, `question_rubric_versions`, and an immutable `question_events`
audit trail (migrations 0012–0014), served by `services/core/internal/question`. A question
carries a stable id, syllabus FK, a **required** `curriculum_map_nodes` FK, a response type
(`multiple_choice`/`short_answer`/`structured_response`), an opaque language tag, the **original**
question prompt, a lifecycle status (`draft`/`verified`/`retired`), and timestamps. A rubric
version carries a stable id, question FK, an immutable positive per-question version number
allocated server-side, a validation-safe `rubric` JSONB structure, maximum marks, a status
(`draft`/`verified`), the creating and reviewing **verified Clerk subjects**, and timestamps.
**The public repository holds schema, code, contracts, docs, and tests only.** Prompts and rubric
structures are original content that exists solely in a runtime database, written by a future
private, approved editorial workflow; nothing is seeded, and no past-paper question, mark scheme,
syllabus text, extract, diagram, or OCR output is stored anywhere. This task performs no AI
generation, no Anthropic call, no OCR, no ingestion, and no question derivation.

Core is the sole authority for a question's **grounding gate**, re-validated inside the write
transaction before any row is written, on **every** question write — create, `PATCH`, rubric-
version create, rubric-version verify, question verify, and question retire: the referenced
curriculum-map node must exist, be `verified`, and belong to the question's syllabus, and the
node's content source must still pass the T-0006 source gate (exists, `approved`,
`catalogue_syllabus_id` equal to the syllabus). `syllabusId` is immutable on a question; the node
link is repointable and re-validates the syllabus match. A question may only be verified when it
has at least one **verified** rubric version. Rubric-version content is immutable at the database
level: a `BEFORE UPDATE OR DELETE` trigger rejects deletes and any change to `question_id`,
`version`, `rubric`, `max_marks`, `created_by`, or `created_at`, so only verification metadata can
change; `UNIQUE (question_id, version)` plus allocation under a row lock on the parent question
keeps numbers unique and monotonic. Retired questions vanish from every reader endpoint (reads are
verified-only). `question_events` mirrors the existing audit tables: append-only, trigger-enforced,
verified-Clerk-subject actor, changed-field **names** only — never a prompt, rubric, or mark
value. Least-privilege permissions `question:read`, `question:create` (draft question + draft
rubric version), `question:verify` (verify rubric, verify question, retire question), and
`question_rubric:read` (rubric listing, which exposes draft rubric structure) are added to the
role matrix: editor gets read/create/rubric-read, reviewer adds verify, admin has all; learner and
unknown are denied. Requests are strictly decoded through an explicit case-sensitive field
allowlist on both `POST` and `PATCH`, so unknown fields, `actorId`/`reviewerId`, `syllabusId`
changes, `status` spoofing, case variants, and trailing JSON all return one stable `400
invalid_json` before any store call. Infrastructure failures always return a single generic
`internal_error`; raw driver or scan text is never forwarded.
**Reason:** Exam Mode needs questions and rubrics that can be authored, reviewed, versioned, and
audited without ever letting copyrighted material into the repository (D-0005) or letting a
question rest on grounding that no longer passes the rights gate (D-0008); the "verified node +
still-approved source" pair is a point-in-time fact about other tables that a bare FK cannot
express, so a single explicit server-side gate is required; immutable numbered rubrics are what
makes marking reproducible and the canonical explanation cache key (`question + syllabus + rubric
+ language + explanation version`, per D-0003) meaningful; the editor/reviewer split mirrors the
existing content-source and curriculum-map surfaces.
**Alternatives:** Ground a question on a syllabus alone (rejected: loses the objective-level
traceability the curriculum map exists to provide, and skips the source gate); mutate a rubric in
place instead of versioning it (rejected: a marked answer could no longer be explained by the
rubric that produced it, and the explanation cache key would be ambiguous); enforce the grounding
gate with a database trigger (rejected: raw `RAISE EXCEPTION` text would leak or force fragile
message parsing — same reasoning as D-0008); let a caller supply the rubric version number
(rejected: races and gaps, and a caller could overwrite history); let the AI service create or
verify questions (rejected: Core stays the single content authority, and this task adds no AI
path at all); seed example questions or rubrics for the D-0004 biology slice (rejected: no source
has completed rights approval, no map node exists, and seeded content in a public repository is
exactly what D-0005 forbids).
**Owner/date:** Claude Code agent, 2026-08-04 (T-0007).

### Update (T-0007 review)

Three review findings were fixed on top of `768c8e2`. All three are additive; no table, column,
role, permission, visibility rule, or source-gate behaviour changed.

1. **A rubric version is bound to the question content it was reviewed against.** A draft question
   could gain a **verified** rubric version and then have its `prompt`, `responseType`, `language`,
   or `curriculumMapNodeId` changed, and `VerifyQuestion` would still accept that rubric — one
   nobody had reviewed against the current wording. Migration `0015` adds
   `questions.content_revision` (starts at 1, incremented by **exactly one** inside the transaction
   of every **successful** draft content update; never moved by `no_changes`, a rejected write, or
   a lifecycle transition; never caller-settable) and `question_rubric_versions.question_revision`
   (the question's revision at the moment the version was created, stamped under the same row lock
   that allocates the version number). A version is **current** only while the two are equal, and a
   question may only be verified when it has a **verified, current** version. `question_revision`
   joins the trigger's immutable set, so no direct SQL rewrite can re-point a stale version at
   current content. An edit **stales** older versions rather than deleting or downgrading them:
   they stay `verified`, immutable, and readable to editorial roles. Because the two failures need
   different fixes, the stale case gets its own stable code, `409
   missing_current_verified_rubric`, instead of reusing the misleading `missing_verified_rubric`.
2. **Rubric JSON keys are matched exactly and case-sensitively.** `ValidateRubric` decoded into a
   Go struct, and Go matches JSON field names case-insensitively, so a rubric documented as
   accepting only `criteria`/`id`/`marks`/`descriptor` in fact accepted `Criteria`, `ID`, `Marks`,
   and `Descriptor` — and, like any struct or map decode, silently kept the last value of a
   duplicated key. Validation is now written against `encoding/json`'s token API against explicit
   key sets, and additionally rejects duplicate keys, non-object criteria, non-string ids, numeric
   marks supplied as strings or fractions, non-string descriptors, and trailing JSON. Responses
   still use the existing stable `invalid_rubric` / `invalid_max_marks` codes and never echo parser
   text. This matches the case-sensitive allowlist the handlers already applied to request bodies.
3. **The optional node filter on `GET /questions` is validated.** `curriculumMapNodeId` was passed
   straight into the `WHERE` clause, so an unknown or foreign node returned `200` with an empty
   list — the same "typo looks like an unauthored map" problem already fixed for `syllabusId` in
   the T-0006 review. It now returns `400 unknown_node` (unknown or malformed) or `400
   mismatched_node` (a real node of another syllabus). The filter is checked more weakly than the
   grounding gate on purpose: a reader may filter by a node that has since been retired, and gets
   an empty list rather than an error. Existing unknown/inactive syllabus behaviour is unchanged,
   and the syllabus is still resolved first.

## D-0010 — Cross-package input-hardening policy

**Status:** Approved
**Decision:** `contentsource`, `catalogue`, and `curriculummap` now enforce the same two input
rules that `question` already enforced (T-0007): (1) every JSON body handler decodes through an
explicit, case-sensitive field allowlist — `map[string]json.RawMessage` first, reject any key not
in the allowlist (covering unknown fields, `actorId`/`reviewerId`, and case variants such as
`SyllabusCode`/`Label`/`SourceUrl`, which `json.Decoder.DisallowUnknownFields` alone does not catch
because Go struct decoding matches field names case-insensitively), then strict-decode into the
destination struct; and (2) every store method that looks up an `{id}` path parameter treats
Postgres `22P02` (invalid text representation — a non-UUID string compared against a UUID column)
the same as `sql.ErrNoRows`, mapping both to the package's existing `ErrNotFound`, never a generic
`500`. Each package keeps its own `decodeStrict`/`isInvalidTextRepresentation`; no shared helper
was extracted across packages.
**Reason:** The T-0006 review already fixed both problems for `curriculummap`'s node-lookup paths
and its `PATCH` handler; T-0007 built `question` with both closed from the start and explicitly
flagged, in its handoff, that `contentsource`, `catalogue`, and `curriculummap`'s remaining POST
bodies and non-map-decoded lookups still had the gaps. Closing them now — before any web editorial
client is built against these APIs — means a client can rely on one consistent error contract
(`400 invalid_json` for bad input, the existing `404`-equivalent domain error for any bad id)
across every package, rather than three packages behaving one way and `question` another.
**Alternatives:** Extract a single shared `decodeStrict`/`isInvalidTextRepresentation` into a
common internal package (rejected: the four packages' existing error-writing helpers
(`writeError`, `writeInvalidJSON`, per-package error-mapping) are not identical, so a shared
decoder would either take several closures per call site or leak assumptions from one package into
another; the per-package duplication is small and each package already had its own copy before
this task); leave the gap and document it only (rejected: it is exactly the gap T-0007's handoff
flagged as the next piece of work, and leaving it open risks a divergent error contract reaching a
future web client); map `22P02` to a distinct new error code instead of the existing `ErrNotFound`
(rejected: a malformed id and a missing id are indistinguishable to a caller with no legitimate
reason to know which, and reusing the existing not-found response keeps the contract stable).
**Owner/date:** Claude Code agent, 2026-08-05 (T-0008).

## D-0011 — Editorial source workflow BFF

**Status:** Approved
**Decision:** `apps/web` gains the first browser-facing surface over the content-source/
catalogue APIs: a protected `/dashboard/editorial/sources` page and a narrow Next.js
route-handler layer (`app/api/editorial/*`, backed by `lib/editorial/core-proxy.ts`) that is
the browser's only path to Core. The browser never calls Core directly. `callCore` accepts a
closed `EditorialOperation` union — not a caller-controlled path/URL — mapping each variant to
exactly one fixed Core method+path template (`GET/POST /content-sources`,
`GET/PATCH /content-sources/{id}`, `POST /content-sources/{id}/approve`,
`POST /content-sources/{id}/reject`, `GET /catalogue/syllabuses`); a supplied `{id}` is
validated against `^[A-Za-z0-9_-]{1,128}$` before interpolation. Each Next.js route file
exports only its allowlisted HTTP method(s), so unsupported verbs get Next's own `405`
structurally. The Core base URL comes only from server-only `SIDUS_CORE_API_URL` (never
`NEXT_PUBLIC_*`); the Clerk session bearer token is obtained server-side per request via
`auth().getToken()` and forwarded to Core, never exposed to or round-tripped through the
browser by this layer. Both are fail-closed: a missing/blank `SIDUS_CORE_API_URL` returns a
generic `503` before any auth check runs; a missing/expired session token returns a generic
`401` before any network call — mirroring `services/core/main.go`'s own
DATABASE_URL-then-Clerk fail-closed ordering. A `POST`/`PATCH` body is checked
(`application/json`, ≤100 KB, syntactically valid JSON) and then forwarded **verbatim** (the
original raw text, never re-parsed and re-serialized) so Core's own strict, case-sensitive
field allowlist (D-0010) stays the sole authority on shape; the approve route ignores any
client body and always sends `{}`. Core's response status/body are passed through unchanged
(already safe/generic per D-0007–D-0010); a `fetch` failure or non-JSON upstream response maps
to a generic `502`, never forwarding the underlying error text, stack trace, or target URL.
Nothing in `lib/editorial/*` or `app/api/editorial/*` logs anything. The web app's own role
check (`lib/editorial/permissions.ts` mirroring `services/core/internal/auth.ParseRole`,
read via `lib/editorial/role.ts` from the verified `sidus_role` Clerk session claim) is
**UI-visibility only** — it decides which controls render (nav entry, page access, review
buttons) and performs zero API calls for `learner`/unknown roles, but every mutation is still
authorized by Core's existing `auth.Protect` (401/403); a stale or wrong web-side role value
can only hide a control Core would have allowed, never show one Core would refuse. This task
adds no new Core role, permission, endpoint, schema, or business rule, and performs no
approval/link of the seeded 0610/5090 sources itself — that stays a human action through the
new UI (D-0005).
**Reason:** A browser-facing editorial surface needs a same-origin boundary that can hold the
Clerk token and Core URL server-side without ever trusting the browser for authorization
(D-0006 precedent: Core is the sole authorization authority); an explicit closed operation
union is what makes "no open proxy" verifiable rather than asserted — a generic
`[...path]`-forwarding route would let any caller reach any Core path/method by construction,
while a fixed union cannot express a request outside the six mounted operations regardless of
input. Raw-body passthrough (vs. parse-and-re-serialize) avoids subtle JSON transform
differences (key order, number precision) between what an editor typed and what Core's strict
decoder sees. Fail-closed ordering and generic error messages mirror the existing Core/AI
precedent (`docs/auth-setup.md` → "Fail-closed configuration") so the whole stack behaves one
way under misconfiguration.
**Alternatives:** A generic `[...path]` proxy route forwarding an allowlisted prefix (rejected:
"allowlisted prefix" is a runtime string check that can drift or be bypassed by encoding
tricks; a closed TypeScript union is checked by the compiler and cannot express an
out-of-union request at all); let the browser call Core directly with a client-managed token
(rejected: contradicts D-0006 — the Clerk secret/JWKS trust boundary and the Core URL would
both need to be public, and CORS would have to open Core to the browser origin); re-serialize
the parsed request body before forwarding (rejected: no behavioral need, and it risks
byte-for-byte drift from what the editor actually submitted vs. what Core validates); check the
caller's role in the BFF and reject before calling Core (rejected: would duplicate Core's
authorization matrix in a second place that could drift from it — D-0006 already establishes
Core as the sole authority, so the web check stays cosmetic); make `app/layout.tsx`'s
role-gated nav entry client-side only via a `/api/editorial/*` call (rejected: forces every
page, including the public home page, to wait on an extra fetch just to decide whether to show
a nav link; reading the already-verified session claim server-side in the existing async layout
is one `auth()` call with no additional round trip).
**Owner/date:** Claude Code agent, 2026-08-05 (T-0009).

**Update (T-0009 review):** `callCore`'s Core-response handling had two gaps closed. (1)
Any Core response with `status >= 500` was previously passed through unchanged like any other
status — Core's existing handlers can return raw Go/database driver error text in a `500`
body, which would have reached the browser verbatim. Now every `5xx` is intercepted before
`resolveRoute`'s caller sees it: the body is read and discarded (never logged, never returned)
and `callCore` throws the same generic `502 upstream_error` / "the editorial service is
temporarily unavailable" regardless of what Core sent. Core `401`/`403` and validation/domain
`4xx` responses are unaffected — only `5xx` is intercepted. (2) The upstream `fetch` call had
no explicit redirect policy, so it defaulted to following redirects — a compromised or
misconfigured Core could have redirected a request to another host and received the forwarded
bearer token. `fetch` now sets `redirect: "error"`, so a Core redirect is never followed and
surfaces as the same generic fetch-failure `502 upstream_unavailable` path, with the token
never sent to the redirect target. No Core, AI, database, migration, auth-role, or UI change;
`docs/editorial-source-workflow.md` and `docs/handoffs/T-0009.md` updated to match.
**Owner/date:** Claude Code agent, 2026-08-05 (T-0009 review).

## D-0012 — Editorial curriculum-map workflow BFF

**Status:** Approved
**Decision:** `apps/web` gains a second browser-facing editorial surface, reusing the T-0009/
D-0011 BFF architecture unchanged: a protected `/dashboard/editorial/curriculum` page and six
new Next.js route handlers (`app/api/editorial/curriculum-map/nodes/*`) that extend the same
closed `EditorialOperation` union in `lib/editorial/core-proxy.ts` with six new variants
(`listCurriculumMapNodes`, `getCurriculumMapNode`, `createCurriculumMapNode`,
`updateCurriculumMapNode`, `verifyCurriculumMapNode`, `retireCurriculumMapNode`), each mapped to
exactly one fixed Core method+path template — the same `callCore`, `requireValidId`,
`readSafeJsonBody`, fail-closed ordering (missing `SIDUS_CORE_API_URL` → 503, invalid
curriculum-map list query → 400 before Clerk/Core network work, missing token → 401), Core-5xx
sanitization, and redirect-refusal apply. `listCurriculumMapNodes` validates required
`syllabusId` with the existing 1–128 character safe resource-ID format and validates `status`
against `draft`/`verified`/`retired`/`all` (or omitted) while resolving its fixed route. The
existing content-source and syllabus routes are reused unmodified — this task adds
no duplicate proxy surface. The page reuses `lib/editorial/role.ts`/`permissions.ts` for the
same UI-visibility-only role gate (`editor`/`reviewer`/`admin` see the workspace;
`reviewer`/`admin` additionally see verify/retire controls; `learner`/unknown see
`AccessDenied` with zero API calls) — Core's existing `curriculum_map:read`/`:create`/`:verify`
permission checks remain the sole authorization authority. The workspace lets an editor pick an
active catalogue syllabus, list its curriculum-map nodes (now every lifecycle status — see
D-0008 "Update (T-0010)"), create/edit a draft node's metadata (kind, code, label, optional
parent, an approved-and-syllabus-linked content source, optional locator), and lets a
reviewer/admin verify or retire a node behind an explicit confirmation step. The `NodeList`
component computes a client-side-only display depth from the loaded node set's `parentNodeId`
chain to show hierarchy; this is presentation only and has no bearing on Core's authoritative
parent/cycle validation. This task creates, verifies, retires, or links **no** node or source
data itself — every action requires an explicit human click through the new UI.
**Reason:** The T-0009 BFF pattern (closed operation union, server-only Core URL and bearer
forwarding, fail-closed config/auth, sanitized 5xx/redirects, UI-only role gate) generalizes
directly to a second Core resource without modification, so reusing it rather than inventing a
second proxy shape keeps exactly one audited "no open proxy" boundary in the codebase. Building
this UI is what exposed that D-0008's original verified-only read rule blocked the very
workflow it was meant to serve (see D-0008 "Update (T-0010)"); fixing that at the Core layer
(rather than, say, having the BFF cache/expose node ids some other way) keeps Core the sole
source of truth for what a caller may read, matching every prior decision in this log.
**Alternatives:** A single generic `/api/editorial/[...path]` proxy shared across both resources
(rejected: reintroduces the exact open-proxy shape D-0011 rejected — a closed union enumerating
every operation, resource included, is what makes "no open proxy" a compiler-checked property);
render the node hierarchy as a true nested tree with server-computed depth (rejected: no
additional business meaning over a flat list with a computed indent for the modest node counts
this workflow handles today, and avoids adding a recursive-rendering component before real usage
shows it is needed — YAGNI); let the web workspace fetch content sources unfiltered by syllabus
and filter purely client-side with no visual cue when the set is empty (rejected: the form
already shows an explicit "no approved source linked to this syllabus" message so an editor is
never left guessing why the selector is empty).
**Owner/date:** Claude Code agent, 2026-08-05 (T-0010).

## D-0013 — Biology vertical-slice scope realignment

**Status:** Approved
**Decision:** Active Biology scope is Cambridge IGCSE Biology 0610 Extended plus exactly one
metadata-only Cambridge International AS & A Level Biology 9700 catalogue syllabus. Cambridge O
Level Biology 5090 becomes retired historical catalogue scope: its existing row, stable id,
timestamps, audit history, source records, associations, curriculum nodes, questions, and rubrics
remain untouched except for catalogue lifecycle status changing to `retired`. The 9700 catalogue
row has board `Cambridge International`, qualification `International AS & A Level`, null track,
display name `Cambridge International AS & A Level Biology`, null curriculum year, and `active`
status. AS and A Level are not split into separate 9700 rows because content-source resolution is
code-based and multiple active rows would be ambiguous. Migration 0016 adds this metadata and
retires 5090 idempotently without changing `updated_at` or writing an invented human audit event.
Its 9700 insert uses conflict-safe `DO NOTHING`: fresh bootstrap gets the approved T-0011 values,
while direct historical SQL re-execution preserves any existing 9700 row's id, timestamps,
human-edited catalogue metadata, lifecycle status, and audit history instead of normalizing it.
No 9700 source, node, question, rubric, syllabus text, objective, assessment material, or derived
content is seeded. Any future 9700 source must be registered through the editorial source workflow
and receive human-verified rights/provenance before approval or downstream use.
**Reason:** Product scope now targets combined Cambridge International AS & A Level Biology 9700;
retaining 5090 catalogue and provenance history preserves referential integrity and auditability,
while one active 9700 row keeps code-based association resolution deterministic.
**Alternatives:** Delete 5090 and its history (rejected: destructive and breaks provenance/audit
continuity); split 9700 into active AS and A Level rows (rejected: ambiguous code-only resolution);
seed a 9700 source or curriculum content (rejected: no human-verified rights/provenance exists).
**Owner/date:** Codex, 2026-08-05 (T-0011).

## D-0014 — Editorial question and rubric workflow BFF

**Status:** Approved
**Decision:** Add protected `/dashboard/editorial/questions` and fixed same-origin Next.js BFF
handlers for existing Core question/rubric operations. Browser calls only `/api/editorial/*`;
`EditorialOperation` maps each operation to one fixed Core method/path, validates resource ids,
question list syllabus/node/status query values, and positive rubric version before auth/fetch,
then forwards only server-acquired Clerk bearer tokens. Existing missing-config/token failures,
JSON envelope limits, redirect refusal, Core-5xx sanitization, and no-logging rules remain shared
with T-0009/T-0010. Web role checks only hide workflow/actions; Core remains sole authorization,
grounding, source-gate, lifecycle, revision, and rubric authority. Editor/reviewer/admin may use
editorial workflow; reviewer/admin alone see verify/retire controls; learner/unknown trigger zero
workflow API calls. Creation node choices come only from Core's `status=verified` curriculum-map
read and are defensively filtered again client-side.

Core question reads receive same minimal editorial correction previously applied to curriculum-map
reads: `GET /questions/{id}` returns any lifecycle state to existing `question:read` holders, and
`GET /questions` defaults to all statuses with optional `draft`/`verified`/`retired`/`all` filter.
This permission remains editor/reviewer/admin-only. No learner-facing route exists. No schema,
migration, role, permission, write-side rule, or runtime record changes.

**Reason:** Verified-only reads made draft discovery/reopening and retired-record review impossible,
blocking browser workflow despite all holders of `question:read` being editorial staff. Closed BFF
union preserves audited no-open-proxy boundary and keeps token/Core URL server-side.

**Alternatives:** Add separate duplicate editorial Core endpoints (rejected: duplicate contract and
authorization surface); cache draft ids in BFF/browser (rejected: Core would stop being source of
truth); expose generic proxy (rejected: caller-controlled target surface); duplicate Core role or
business rules in web (rejected: drift and false authority).

**Owner/date:** Codex, 2026-08-05 (T-0012).

## D-0015 — Deterministic MCQ delivery schema

**Status:** Approved
**Decision:** Add nullable JSONB `questions.options` through additive, rerunnable migration 0017.
Options are ordered original question content, accepted only for `multiple_choice`, with 2–6 exact
`{id,label}` objects; IDs are stable, unique, non-blank, and bounded to 64 Unicode code points,
labels are non-blank and bounded to 1,000. Existing rows remain valid with NULL. Every option
change is one question content update and one revision increment; MCQ-to-non-MCQ clears options in
same update. Extend rubric JSON with exact optional `answerKey:{correctOptionId}`. Core requires it
for MCQ rubric creation, rejects it for non-MCQ, and matches ID to current options while holding
question row lock. Existing immutable rubric JSON and question-revision rules make answer key
versioned and stale older versions after any question content edit. Strict request token decoding
also rejects top-level duplicate keys before store mutation. Editorial BFF keeps same closed routes
and verbatim body forwarding; draft MCQ UI edits/reorders options and selects answer only from
current options. No learner endpoint exists; future learner projection must omit rubric and answer
key. Short/structured responses remain criteria-only.

**Reason:** Options belong to delivered question wording/order, while correct choice belongs to
reviewed marking truth. Separating them preserves existing revision and rubric immutability model,
allows deterministic future delivery, and prevents label copies or mutable question rows from
becoming canonical answers.

**Alternatives:** Store correct flag on each option (rejected: question edit could mutate reviewed
answer truth); separate correct-answer column on questions (rejected: bypasses rubric versioning);
invent exact-match text answers (rejected: marking semantics are not designed); add learner route
now (rejected: authorization/projection/session scope belongs to later task); require non-null
options at database level (rejected: existing MCQ rows must remain valid with NULL after additive
migration).

**Owner/date:** Codex, 2026-08-06 (T-0013).

## D-0016 — Explicit canonical rubric selection

**Status:** Approved
**Decision:** Add nullable `questions.canonical_rubric_version_id` through additive, rerunnable
migration 0018 with no backfill. `POST /questions/{id}/verify` requires exact
`{"rubricVersion": positiveInteger}`. Core locks question then selected owned rubric in one
transaction, rechecks grounding/source gate, requires draft question plus verified rubric stamped
at current content revision, and atomically stores verified status, canonical rubric FK, and
names-only audit event. Core never selects latest or replaces canonical automatically. Historical
verified rows left null by migration may be repaired once through reviewer/admin-only
`POST /questions/{id}/canonical-rubric` using same strict selection and eligibility checks; draft,
retired, foreign, draft-rubric, stale-rubric, and already-selected cases fail safely. Editorial
contracts/reads expose nullable canonical id. Fixed BFF operation and role-gated UI require
selection plus confirmation and show marker. No learner route exists; future learner projection
must omit canonical id, rubric, and answer key.

**Reason:** Multiple verified current rubric versions may coexist. Automatically choosing highest
or latest version makes future delivery/marking depend on mutable query ordering rather than
reviewer intent. Explicit immutable choice makes pairing reproducible while preserving historical
rows without invented decisions.

**Alternatives:** Auto-select latest verified rubric (rejected: violates approved reviewer intent
and can silently change behavior); backfill historical verified rows (rejected: invents selection);
store only per-question version number (rejected: weaker referential integrity than rubric-row FK);
allow reviewer replacement (rejected: breaks stable verified lifecycle and reproducibility); add
learner delivery now (rejected: separate projection/auth/session scope).

**Owner/date:** Codex, 2026-08-06 (T-0014).

## Decision template

```md
## D-XXXX — Title
**Status:** proposed | approved | superseded
**Decision:**
**Reason:**
**Alternatives:**
**Owner/date:**
```
