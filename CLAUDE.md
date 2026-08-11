# Sidus agent guide

Read before planning, editing, testing, or committing.

## Mission

Build Sidus: academic preparation platform. First vertical slice: Cambridge IGCSE Biology 0610 Extended and Cambridge International AS & A Level Biology 9700.

## Read order

1. `docs/tasks/active.md`
2. `docs/decisions.md`
3. `docs/agent-workflow.md`
4. Relevant architecture and content documents

## Architecture

- `apps/web`: Next.js + TypeScript PWA.
- `services/core`: Go high-traffic API.
- `services/ai`: Python/FastAPI AI, OCR, ingestion, marking.
- `packages/shared`: cross-service contracts.
- PostgreSQL is system of record. Redis/object storage/OpenSearch come later.
- Anthropic only: Haiku routine tasks; Sonnet complex marking.
- Canonical explanation cache key: `question + syllabus + rubric + language + explanation version`.

## Non-negotiable content rules

- Never commit PDFs, books, extracted text, diagrams, screenshots, past papers, mark schemes, or lightly rewritten questions.
- Use only source metadata and reviewed mappings until rights approval exists.
- Content ingestion blocks every source except `approved`.
- Original questions link to syllabus/objective IDs, not copied source wording.

## Working rules

- Do not guess. Record missing detail under `Open questions` or `Blockers` in active task.
- Work only task scope. Create a new task entry before scope expansion.
- Preserve unrelated files and user changes. Stage only own files.
- Run relevant checks. Record command and result in task handoff.
- Update task status and create handoff before commit/hand-off.
- Never overwrite another agent's active task. Ask user or create separate task.

## Commands

```sh
npm --prefix apps/web run typecheck
npm --prefix apps/web run test
npm --prefix apps/web run build
docker run --rm -v "$(pwd)/services/core:/app" -w /app golang:1.22-alpine go test ./...
cd services/ai && python -m pytest
```

## Current state

- T-0031 released: white-primary Sidus Observatory alignment across private uploads, mixed answer
  controls, shared primitives, and Exam dialog accessibility. See `docs/handoffs/T-0031.md`.

- T-0030 released: private admin PDF quarantine and Sonnet-ready review-intake foundation. Files
  remain private, bounded, unserved, and scanner-gated; no OCR/extraction/model invocation exists
  yet. See `docs/handoffs/T-0030.md`.

- T-0029 released: learner Practice and Exam support MCQ, multi-select, exact short answer,
  numeric answer, and written responses. Deterministic types mark only after submission; written
  responses remain pending review and receive no invented score. See `docs/handoffs/T-0029.md`.

- T-0028 released: learner Practice and Exam now expose only human-readable Modules through a
  learner-safe eligible-only projection. Raw curriculum identifiers are not shown or requested.
  Both modes support All available or any positive count up to live availability. See
  `docs/handoffs/T-0028.md`.

- Foundation commit: `e7e2179`.
- Biology syllabus/provenance commit: `4cfb5d3`.
- Curriculum catalogue (multi-subject) released in T-0004. Core owns the metadata-only
  `subjects`/`syllabuses` registry; see `docs/curriculum-catalogue.md` and D-0007.
- Provenance-confirmed catalogue linking released in T-0005; see
  `docs/provenance-catalogue-linking.md`. The seeded 0610 content source still needs a human
  editor/admin `PATCH` to link. The historical seeded 5090 source remains preserved but its
  retired catalogue syllabus cannot resolve for a new active association. No 9700 source exists;
  any future one must enter through the editorial source registry with human-verified rights and
  provenance.
- Curriculum-map foundation (metadata-only topic/objective/practical-skill/assessment-rule
  infrastructure) released in T-0006; see `docs/curriculum-map.md` and D-0008. No map data is
  seeded — a human must first approve and link a content source, then author map content via a
  future private approved workflow. Four review findings were fixed on top of `b1677cb`
  (strict PATCH decoding, source gate re-validated on every node write, syllabus validation on
  map list, real ancestor row locking) — see D-0008 "Update (T-0006 review)".
- Original question and versioned rubric foundation (private, metadata/code-only infrastructure
  for a future Exam Mode) built in T-0007; see `docs/question-rubric-model.md` and D-0009. Core
  owns `questions`/`question_rubric_versions`/`question_events` (migrations 0012–0015). Every
  question must trace to a **verified** curriculum-map node whose approved source still passes the
  T-0006 source gate — re-checked on every write. No question, rubric, or map data is seeded, and
  no AI/OCR/ingestion path is involved. Three review findings were fixed on top of `768c8e2`
  (question `content_revision` so a question can only be verified with a rubric verified against
  its **current** content, case-sensitive rubric JSON keys, validated node filter on
  `GET /questions`) — see D-0009 "Update (T-0007 review)". Released 2026-08-05.
- Cross-package API input hardening released in T-0008: `contentsource`, `catalogue`, and
  `curriculummap` now match `question`'s strict-decode allowlist (rejects case-variant field
  names, `actorId`/`reviewerId`, unknown fields, trailing JSON, and literal JSON `null` bodies)
  and map malformed (non-UUID) `{id}` path parameters to the existing not-found response instead
  of a `500` — see D-0010 and `docs/handoffs/T-0008.md`. No business rule, schema, or seed
  content changed. Released 2026-08-05.
- Private editorial source workflow released in T-0009: `apps/web` gains a protected
  `/dashboard/editorial/sources` page and a narrow BFF (`app/api/editorial/*`, backed by
  `lib/editorial/core-proxy.ts`) that is the browser's only path to the content-source/
  catalogue APIs — see `docs/editorial-source-workflow.md` and D-0011. Fixed operation
  allowlist (no open proxy), server-only `SIDUS_CORE_API_URL` and Clerk bearer forwarding,
  fail-closed on missing config/token. Core stays the sole authorization authority; the web
  role check only hides/shows controls. No Core/AI/migration/business-rule change, and no
  approval/link of seeded sources was performed (that is a human action through the new UI). A
  review fix on top of `15d5936` (commit `f31a8a5`) sanitizes any Core
  `5xx` response to a generic `502` before it reaches the browser and sets the upstream `fetch`
  to fail closed on redirects (`redirect: "error"`) — see D-0011 "Update (T-0009 review)" and
  `docs/handoffs/T-0009.md`. T-0009 released and moved to `docs/tasks/history.md` as `done`.
- Private editorial curriculum-map workflow released in T-0010: `apps/web` gains
  a protected `/dashboard/editorial/curriculum` page reusing the T-0009 BFF pattern (six new
  `EditorialOperation` variants in `lib/editorial/core-proxy.ts`, six new route handlers under
  `app/api/editorial/curriculum-map/nodes/*`) so editors can author/edit draft nodes and
  reviewers/admins can verify/retire them, all behind the existing UI-only role gate. Includes a
  tiny, essential, user-approved Core contract change: `GET /curriculum-map/nodes` and
  `GET /curriculum-map/nodes/{id}` now return nodes of any lifecycle status (not verified-only)
  to a `curriculum_map:read` holder, since that permission is already restricted to
  editor/reviewer/admin — see D-0008 "Update (T-0010)", D-0012, and
  `docs/editorial-curriculum-workflow.md`. No node/source data created, approved, linked,
  verified, or retired automatically. Full release validation passed on 2026-08-05; see
  `docs/handoffs/T-0010.md`. T-0010 moved to `docs/tasks/history.md` as `done / released`.
- Biology catalogue scope realigned in T-0011: 0610 Extended and one combined metadata-only 9700
  syllabus are active; 5090 remains present as retired historical catalogue metadata. Migration
  creates no 9700 content source, node, question, or rubric and changes no existing provenance or
  content records. Full release validation passed on 2026-08-05; see D-0013 and
  `docs/handoffs/T-0011.md`. T-0011 moved to `docs/tasks/history.md` as `done / released`.
- Private editorial question/rubric workflow built in T-0012: `apps/web` gains protected
  `/dashboard/editorial/questions` and a fixed-operation same-origin BFF for Core T-0007 question
  and rubric routes. Authorized editorial question reads now expose draft/verified/retired states
  with optional status filter so staff can reopen drafts; write rules, grounding/source gates,
  revisions, rubric versioning, permissions, schema, migrations, and runtime data remain unchanged.
  See `docs/editorial-question-rubric-workflow.md`, D-0014, and `docs/handoffs/T-0012.md`.
  Full release validation passed on 2026-08-05. T-0012 moved to `docs/tasks/history.md` as
  `done / released`.
- Deterministic MCQ delivery schema built in T-0013: additive migration 0017 adds nullable
  question-owned `options`; Core enforces exact 2–6 stable-ID/original-label option shape and
  rubric-versioned `answerKey.correctOptionId` matched to current options. Draft editorial UI can
  add/remove/reorder options and select current correct option. Existing grounding, revisions,
  immutable rubrics, audit-names-only, roles, and fail-closed BFF remain authoritative. No learner
  endpoint, attempt/session, marking, AI, explanation, timer, Exam Mode, or seeded content exists.
  Full release validation passed on 2026-08-06; see `docs/question-delivery-schema.md`, D-0015, and
  `docs/handoffs/T-0013.md`. T-0013 moved to `docs/tasks/history.md` as `done / released`.
- Explicit canonical rubric selection built in T-0014: additive migration 0018 adds nullable
  `questions.canonical_rubric_version_id` with no backfill. Question verification now requires a
  reviewer/admin to select one owned, verified rubric for current content revision; Core locks and
  writes status, selection, and names-only audit atomically after grounding recheck. Historical
  verified null rows have one reviewer/admin-only repair endpoint; selection replacement and
  automatic latest-version choice are forbidden. Editorial BFF/UI expose selection and marker.
  No learner route exists; future learner projection must omit canonical id, rubric, and answer
  key. Full release validation passed on 2026-08-06; see `docs/canonical-rubric-selection.md`,
  D-0016, and `docs/handoffs/T-0014.md`. T-0014 moved to `docs/tasks/history.md` as
  `done / released`.
- Learner-safe verified-question delivery foundation released in T-0015: new Core
  `services/core/internal/learner` package (own types, no import of `question`), `GET
  /learner/questions` and `GET /learner/questions/{id}`, and a new `learner_question:read`
  permission held by every recognized role. A question is returned only while verified, its
  canonical rubric is verified and current, its node is verified, and its source is approved
  and catalogue-linked — re-checked on every read. Response is the explicit `LearnerQuestion`
  projection only: no status, canonical rubric id, rubric, answer key, marks, audit data, actor
  identity, timestamps, or source metadata. New web BFF (`apps/web/lib/learner/*`, separate
  `LearnerOperation` union) and `/dashboard/practice` screen let any signed-in recognized role
  read eligible questions; MCQ options are selectable but nothing submits, marks, reveals an
  answer, times, or calls AI. No schema change, no learner-facing catalogue/curriculum-map
  browse endpoint, no Exam Mode. See `docs/learner-question-delivery.md`, D-0017, and
  `docs/handoffs/T-0015.md`. Two review findings were fixed on top of `d60f6e8` (canonical-rubric
  eligibility join now also requires `rv.question_id = q.id`; new `GET /learner/syllabuses`
  route, reusing `learner_question:read`, backs an accessible syllabus `<select>` on
  `/dashboard/practice` in place of a raw-UUID text field) — see D-0017 "Update (T-0015 review)".
  Full release validation passed on 2026-08-06. T-0015 moved to `docs/tasks/history.md` as
  `done / released`; implementation commits remain unchanged.
- Practice Mode MCQ marking released in T-0016: immutable MCQ rubrics require complete verified
  canonical feedback for current options; additive migration 0019 adds owner-bound, pinned,
  one-shot learner attempts and append-only names-only events. Core marks all-or-zero under row
  lock from exact pinned canonical rubric/revision and returns only learner-safe result fields.
  Separate fixed-operation learner BFF routes and `/dashboard/practice` provide explicit submit,
  score, selected/correct labels, and every canonical explanation. Historical rubric rows are not
  backfilled; no content/seed, fallback, AI, timer, paper/session flow, or Exam Mode exists. See
  `docs/practice-mcq-marking.md`, D-0018, and `docs/handoffs/T-0016.md`. Full release validation
  passed on 2026-08-06. T-0016 moved to `docs/tasks/history.md` as `done / released`;
  implementation commits remain unchanged.
- Sidus Observatory visual design system (light-first white + blue-ink, dark-mode navy, original
  `A*` logo, IBM Plex fallback-stack typography, central `lib/design/status.ts` and
  `lib/design/option-state.ts` helpers) released in T-0017 and applied to the landing page,
  signed-in shell/nav, Practice Mode, and all three editorial workspaces in `apps/web`.
  Presentation only — no Core/AI/BFF/database/route/dependency change. See
  `docs/sidus-observatory-design-system.md`, D-0019, and `docs/handoffs/T-0017.md`.
- Three Gemini-audit findings on T-0017 fixed on top of `26a4a8a`: Practice Mode's MCQ options
  are now a full roving-tabindex ARIA radiogroup with arrow/Home/End/Space/Enter keyboard support
  (`apps/web/app/dashboard/practice/question-list.tsx` + new `question-list.test.tsx`); the mobile
  (`≤40rem`) top nav scrolls its link strip in a single row instead of wrapping into a tall header
  (`apps/web/components/nav/nav.module.css`); and hard-coded spacing/sizing values across
  `nav.module.css`, Practice Mode's `styles.module.css`, and `Logo.module.css` now resolve to
  design tokens, with two documented exceptions (border-compensated MCQ option padding kept exact
  via `calc()`, and typography, which has no token scale yet). No Core/AI/BFF/database/route/
  business-rule/dependency change. See D-0019's update in `docs/decisions.md` and
  `docs/handoffs/T-0017.md` "Update (T-0017 review fix)". Follow-up fixes also suppress expected
  theme hydration differences at root HTML and remove all CSS gradients through a tokenized,
  reduced-motion-safe Skeleton opacity pulse. Full release validation passed on 2026-08-06;
  T-0017 moved to `docs/tasks/history.md` as `done / released`; implementation commits remain
  unchanged.
- Licensed-adaptation question provenance built in T-0018: every new question explicitly chooses
  `original` or `licensed_adaptation`; additive migration 0020 leaves historical rows unclassified
  and adds immutable metadata-only provenance plus names-only audit. Licensed create, verification,
  learner question delivery, attempt creation, and feedback submission recheck approved,
  rights-complete, catalogue-linked source with no fallback. Editorial UI uses approved source
  metadata picker and locator only; learner types/responses remain provenance-free. Human must
  enter written licence evidence and approve it before use; software never invents licence facts or
  handles source material. See `docs/licensed-adaptation-provenance.md`, D-0020, and
  `docs/handoffs/T-0018.md`. Full release validation passed on 2026-08-07; T-0018 moved to
  `docs/tasks/history.md` as `done / released`; implementation commit `50ea7b2` unamended.
- Private licensed-source reference URI released in T-0021: `content_sources.sourceUrl` now
  accepts either an unchanged `http`/`https` URL or exactly one new private form,
  `sidus-private://licensed/cambridge-international/9700/{session}/{component}`, via one shared,
  strictly anchored validator used by both create and update (create previously had no format
  check at all). A review fix canonicalizes storage by trimming `sourceUrl` once after validation
  succeeds. The URI is metadata identity only — never dereferenced, fetched, or logged — and the
  existing rights-approval gate, error codes, and roles are unchanged; no schema/migration change
  and no learner surface ever exposes `sourceUrl`. See `docs/decisions.md` D-0021 and
  `docs/handoffs/T-0021.md`. Full release validation passed on 2026-08-07; T-0021 moved to
  `docs/tasks/history.md` as `done / released`; implementation commits `cd6b496`/`ed67db9`
  unamended.
- Local-only HTTPS Core test environment released in T-0023: an isolated `docker-compose.
  local-import.yml` stack (project `sidus-local-import`; own Postgres, migration run, Core, and
  a Caddy HTTPS reverse proxy bound `127.0.0.1:443` only) lets the private T-0022 pending-source
  import tool be exercised against a real running Core over real TLS with real Clerk-gated auth.
  Private dev TLS CA/cert live only under `D:\Sidus-private-content\local-dev`, never this repo.
  T-0022's `api_client.py` gained one optional, additive `SIDUS_CORE_CA_BUNDLE`-backed parameter;
  unset behavior is byte-for-byte unchanged. No production Compose, auth policy, schema, learner,
  or BFF behavior changed, and no source/approval/question/rubric/node/attempt row was created —
  the documented one-record smoke test and the full 489-record `--apply` are separate, explicit,
  later human actions. See `docs/decisions.md` D-0022, `docs/local-import-test-environment.md`,
  and `docs/handoffs/T-0023.md`. Full release validation passed on 2026-08-08; T-0023 moved to
  `docs/tasks/history.md` as `done / released`; implementation commit `1b3cad6` unamended.
- Local TLS certificate-generation defect fixed in T-0024: T-0023's local HTTPS test-environment
  doc had documented a CA with no `keyUsage` extension, which `ssl.create_default_context` (the
  T-0022 client's `SIDUS_CORE_CA_BUNDLE` path) rejects. `docs/local-import-test-environment.md`
  now generates the CA/leaf cert with explicit openssl config extensions; only the private cert
  files under `D:\Sidus-private-content\local-dev` were regenerated. No Compose/Caddy/Core/AI/
  client code, schema, or business rule changed. See `docs/decisions.md` D-0022 "Update
  (T-0024)" and `docs/handoffs/T-0023.md` "Update (T-0024)". Independent review passed and full
  release validation passed on 2026-08-10; T-0024 moved to `docs/tasks/history.md` as
  `done / released`; implementation commit `5e8b35f` unamended.
- T-0025 local authenticated browser E2E harness released: Playwright uses real Clerk session
  captures stored only under `D:\Sidus-private-content\e2e`, drives the local UI → BFF → private-TLS
  Core flow using opaque runtime-only synthetic fixtures, and never enables an auth bypass or
  weakens TLS. Signed-out and full authenticated E2E passed 10/10 on 2026-08-11 with private
  captures for `admin`, `learner`, and `unknown`. See `docs/e2e-harness.md`, D-0023, and
  `docs/handoffs/T-0025.md`.
- T-0026 local Exam Mode MVP released: `/dashboard/exam` composes existing learner-safe
  syllabus/question/attempt routes without Core or schema changes. It holds paper order, answers,
  flags, browser-local timer, and retry state in memory only; timer expiry reads latest answers
  and continues through accessible submission confirmation. Results explicitly show an
  answered-only score because unanswered questions have no Core attempt/max-mark response.
  Playwright covers opaque two-question full flow and passed on documented local web/Core runtime
  on 2026-08-11. No source/question content is seeded or imported. See
  `docs/handoffs/T-0026.md`.
- T-0028 learner Module selection and availability-aware count is in review: a learner-safe
  `GET /learner/modules` route exposes only modules with at least one live eligible question;
  Practice and Exam use Module selectors and All/custom positive counts without a 10-question
  cap. Response-type expansion remains separate; see `docs/handoffs/T-0028.md`.
