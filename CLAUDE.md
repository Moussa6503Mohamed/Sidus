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
  `lib/design/option-state.ts` helpers) built in T-0017 and applied to the landing page, signed-in
  shell/nav, Practice Mode, and all three editorial workspaces in `apps/web`. Presentation only —
  no Core/AI/BFF/database/route/dependency change. See `docs/sidus-observatory-design-system.md`,
  D-0019, and `docs/handoffs/T-0017.md`. T-0017 is at status `review` (implementation complete,
  not released, not pushed) — not yet moved to `docs/tasks/history.md`.
- Three Gemini-audit findings on T-0017 fixed on top of `26a4a8a`: Practice Mode's MCQ options
  are now a full roving-tabindex ARIA radiogroup with arrow/Home/End/Space/Enter keyboard support
  (`apps/web/app/dashboard/practice/question-list.tsx` + new `question-list.test.tsx`); the mobile
  (`≤40rem`) top nav scrolls its link strip in a single row instead of wrapping into a tall header
  (`apps/web/components/nav/nav.module.css`); and hard-coded spacing/sizing values across
  `nav.module.css`, Practice Mode's `styles.module.css`, and `Logo.module.css` now resolve to
  design tokens, with two documented exceptions (border-compensated MCQ option padding kept exact
  via `calc()`, and typography, which has no token scale yet). No Core/AI/BFF/database/route/
  business-rule/dependency change. See D-0019's update in `docs/decisions.md` and
  `docs/handoffs/T-0017.md` "Update (T-0017 review fix)". T-0017 remains at status `review` — not
  released, not pushed.
- See `docs/tasks/active.md` / `docs/tasks/history.md`.
