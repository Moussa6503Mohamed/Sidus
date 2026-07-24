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

## Decision template

```md
## D-XXXX — Title
**Status:** proposed | approved | superseded
**Decision:**
**Reason:**
**Alternatives:**
**Owner/date:**
```
