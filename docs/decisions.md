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

## Decision template

```md
## D-XXXX — Title
**Status:** proposed | approved | superseded
**Decision:**
**Reason:**
**Alternatives:**
**Owner/date:**
```
