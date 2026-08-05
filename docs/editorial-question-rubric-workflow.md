# Editorial question and rubric workflow

T-0012 adds private browser workflow over Core T-0007. It creates no runtime data during build,
test, migration, startup, or deployment. Every create, edit, verify, or retire requires explicit
human action in authenticated UI.

## Route and authority model

Browser uses only `/api/editorial/*`. Next.js BFF obtains Clerk token server-side and forwards it
to fixed Core route selected from closed `EditorialOperation` union. Core URL comes only from
server-only `SIDUS_CORE_API_URL`. Core remains sole authority for role permissions, verified-node
grounding, approved/linked/matching source gate, question lifecycle, content revisions, rubric
version allocation/immutability/validation, and reviewer identity.

Actual Core routes used:

| Core route | Method | Purpose |
| --- | --- | --- |
| `/questions?syllabusId=...&curriculumMapNodeId=...&status=...` | GET | list questions; node/status optional |
| `/questions` | POST | create draft question |
| `/questions/{id}` | GET, PATCH | get question; edit draft |
| `/questions/{id}/verify` | POST | verify question |
| `/questions/{id}/retire` | POST | retire question |
| `/questions/{id}/rubric-versions` | GET, POST | list or append rubric version |
| `/questions/{id}/rubric-versions/{version}/verify` | POST | verify rubric version |

Existing `/catalogue/syllabuses` and
`/curriculum-map/nodes?syllabusId=...&status=verified` BFF operations supply selectors.

## BFF allowlist and validation

| Browser route | Exported methods | Fixed Core operation |
| --- | --- | --- |
| `/api/editorial/questions` | GET, POST | list/create questions |
| `/api/editorial/questions/{id}` | GET, PATCH | get/update question |
| `/api/editorial/questions/{id}/verify` | POST | verify question |
| `/api/editorial/questions/{id}/retire` | POST | retire question |
| `/api/editorial/questions/{id}/rubric-versions` | GET, POST | list/create versions |
| `/api/editorial/questions/{id}/rubric-versions/{version}/verify` | POST | verify version |

No catch-all route or caller-supplied Core path/URL exists. Resource ids use shared safe segment
format (1–128 alphanumeric/underscore/hyphen characters). Rubric version must be positive decimal
PostgreSQL integer. Question list requires valid `syllabusId`; optional node id uses same
validation; optional status is exactly `draft`, `verified`, `retired`, or `all`. Route resolution
validates these before Clerk lookup or fetch. JSON mutations require `application/json`, maximum
100 KB raw body, and valid JSON before fetch; body is forwarded verbatim for Core strict decoding.
Lifecycle POST handlers ignore browser body and send fixed `{}`.

Fail closed behavior remains shared: missing Core URL → generic 503 before auth; missing token →
generic 401 before fetch; network failure or redirect → generic 502; every Core 5xx body is consumed
and discarded then generic 502 returned; non-JSON upstream body → generic 502. No request body,
prompt, rubric, token, Core URL, or exception detail is logged.

## UI workflow and states

`/dashboard/editorial/questions` is available to editor/reviewer/admin. Learner/unknown sees denied
state and mounts no workspace, producing zero workflow API calls. Reviewer/admin alone see rubric
verify, question verify, and retire controls; this is visibility only, never authorization.

- Active syllabus selector and optional verified-node question filter.
- Creation/edit node choices use verified nodes only. Core rechecks grounding on every write.
- Question list shows lifecycle, response type, language, node, and content revision.
- Empty create form supports `multiple_choice`, `short_answer`, `structured_response`, language,
  prompt, and node. Provenance warning requires original editorial content only.
- Draft-only edit sends changed fields. Successful content change receives Core-incremented revision.
- Rubric workspace lists version, draft/verified state, question revision, maximum marks, and
  current/stale comparison against selected question revision.
- Empty structured criteria editor supports id, positive integer marks, optional nonblank
  descriptor, unique ids, 1–200 criteria, per-criterion maximum 1000, and exact max-mark sum.
  Client validation mirrors safe shape; Core remains authority.
- Verify/retire actions require explicit confirmation. Loading, empty, error/retry, locked,
  denied, and horizontally scrollable mobile table states are present.

Core domain errors receive actionable browser messages for node/source grounding regression,
lifecycle restriction, missing verified rubric, missing current rubric after revision change,
invalid rubric/max marks, duplicate rubric allocation, and no-change updates.

## Minimal Core read correction

Existing `question:read` belongs only to editor/reviewer/admin. T-0007 list/get forced verified-only,
so author could not rediscover draft and reviewer could not audit retired record. T-0012 changes
only reads: list defaults to all lifecycle states and accepts status filter; get returns any status.
Syllabus/node validation and every write-side rule remain unchanged. No public/learner read added.

## Content safety

Repository contains empty forms, schemas, validation, generic labels, and synthetic identifiers
only. No question prompt, rubric descriptor/answer, source material, PDF/extract, syllabus wording,
past-paper item, mark scheme, diagram, fixture, default, or seed was added. Tests generate opaque
runtime strings where prompt-shaped values are necessary. No runtime database operation is run by
application setup or validation.
