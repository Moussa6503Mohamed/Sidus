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

**Status:** Approved
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

## Decision template

```md
## D-XXXX — Title
**Status:** proposed | approved | superseded
**Decision:**
**Reason:**
**Alternatives:**
**Owner/date:**
```
