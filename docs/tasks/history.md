# Task history

## T-0032 — Persistent assessment sessions

**Status:** done / released
**Implementation commits:** `f3742a4`, `8733340`, `29210fc`
**Release validation:** server session integration and HTTP contract tests pass locally. Sessions
provide immutable learner-safe paper snapshots, server deadlines, autosave versions, idempotent
finalization, and mixed answers. Live browser/API E2E remains deferred until final API integration
phase. See `docs/handoffs/T-0032.md`.

## T-0031 — Brand-kit alignment

**Status:** done / released
**Implementation commits:** `c38890d`, `472f1a4`
**Release validation:** independent UI review approved after accessible Exam radiogroup fix.
White-primary Observatory surfaces, branded upload and mixed answer controls, token cleanup, and
accessible Exam dialog released. Web 353/353, typecheck, production build, and diff checks passed.
Plex local font assets remain a documented future asset task. See `docs/handoffs/T-0031.md`.

## T-0030 — Private admin upload and AI review intake foundation

**Status:** done / released
**Implementation commits:** `0cd76e5`, `65864ce`
**Release validation:** independent security review approved. Admin-only private PDF quarantine,
bounded multipart intake, scanner-attestation gate, immutable audit trail, and Sonnet-ready
metadata review contract released. No OCR, source extraction, model call, or public file serving.
Web 352 tests, Go tests, Python 20 tests, type/build/contracts/Compose/diff checks passed.
See `docs/handoffs/T-0030.md`.

## T-0029 — Learner answer-type expansion

**Status:** done / released
**Implementation commits:** `84cbcb3`, `1d1f95f`, `02cbd69`
**Release validation:** independent review approved. Practice and Exam support deterministic MCQ,
multi-select, exact short answer, and numeric answers, plus written answers that remain pending
review. Web 348/348, Core unit/integration, 21 migrations plus idempotent rerun, Python, shared
TypeScript, Compose, and diff checks passed on 2026-08-11. See `docs/handoffs/T-0029.md`.

## T-0028 — Learner module selection and flexible question counts

**Status:** done / released
**Implementation commit:** `0d0ee6a`
**Release validation:** independent review approved; Web Vitest 343/343, typecheck, production
build, Core unit/integration, strict shared TypeScript, Compose validation, and `git diff --check`
passed on 2026-08-11. Learner Practice and Exam now use a safe Module selector, not raw internal
curriculum identifiers, and support All available or any positive count up to current availability.
See `docs/handoffs/T-0028.md`.

## T-0026 — Local Exam Mode MVP

**Status:** done / released
**Implementation commits:** `0206568`, `665addc`, `8a7871e`
**Release validation:** Web Vitest 333/333, typecheck, production build, and authenticated local
Playwright 10/10 passed on 2026-08-11. Exam Mode is browser-local only: no durable session,
authoritative timer, or real source content. See `docs/handoffs/T-0026.md`.

## T-0025 — Local authenticated browser E2E harness

**Status:** done / released
**Implementation commits:** `13052b1`, `57702fb`
**Release validation:** real local Clerk captures for admin/learner/unknown, strict private-CA TLS,
and full Playwright 10/10 passed on 2026-08-11. Storage state and artifacts remain outside Git.
See `docs/handoffs/T-0025.md`.

## T-0024 — Local TLS certificate generation defect fix (D-0022 update)

**Status:** done / released
**Owner:** Claude Code agent
**Implementation commit:** `5e8b35f1097392e4b6c71d6fa0447f2e7f398407`
**Depends on:** T-0023 (done/released — this fixes a defect in its cert-generation doc)

### Goal

Fix a certificate-generation defect in T-0023's local HTTPS test-environment doc: the documented
CA lacked a `keyUsage` extension, which `ssl.create_default_context` (the T-0022 client's
`SIDUS_CORE_CA_BUNDLE` path) rejects with `CERTIFICATE_VERIFY_FAILED: CA cert does not include
key usage extension`.

### Delivered

- `docs/local-import-test-environment.md`: cert-generation section now uses an explicit `ca.cnf`
  for the root CA (`basicConstraints = critical, CA:true`; `keyUsage = critical, keyCertSign,
  cRLSign`; `subjectKeyIdentifier = hash`) and a two-section leaf config — the CSR requests only
  `subjectAltName`; the full extension set (`basicConstraints = critical, CA:false`; `keyUsage =
  critical, digitalSignature, keyEncipherment`; `extendedKeyUsage = serverAuth`; `subjectAltName`
  with `DNS:localhost`/`IP:127.0.0.1`; `authorityKeyIdentifier = keyid,issuer`) is applied only at
  the `x509 -req -CA ... -extensions v3_leaf` signing step.
- `docs/decisions.md` D-0022 "Update (T-0024)": root cause and fix record.
- `docs/handoffs/T-0023.md` "Update (T-0024)": bug, root cause, fix, and validation table.
- Regenerated `ca.pem`/`ca-key.pem`/`server-cert.pem`/`server-key.pem` under
  `D:\Sidus-private-content\local-dev` with the corrected commands (private, not tracked).

### Constraints

- No `docker-compose.local-import.yml`, Caddyfile, Core, AI, or T-0022 client code change.
- No schema, migration, or business-rule change.
- No `content_sources`, approval, question, rubric, node, or attempt row created.
- No PDF, book, extracted text, diagram, screenshot, past paper, or mark scheme read or touched.

### Open questions / blockers

None.

### Release validation (final pass, 2026-08-10)

| Command | Result |
| --- | --- |
| Independent review (security-reviewer agent, doc-only cert-generation diff) | Pass — PASS verdict, no CRITICAL/HIGH/MEDIUM findings |
| Git/staged audit (`git status --short`, `git show --name-only`, secret grep) | Pass — only the 5 intended files changed in commit `5e8b35f`; pre-existing untracked local files untouched; no secret/key material in diff |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` / `-f docker-compose.local-import.yml --env-file .env.local-import.example config` | Pass / Pass / Pass |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run test` (Vitest) | Pass — 289/289, 32 files |
| `npm --prefix apps/web run build` | Pass — 29 routes intact |
| `npx tsc --strict --noEmit packages/shared/src/contracts.ts` | Pass |
| `gofmt -l . && go build ./... && go vet ./... && go test ./...` (Docker `golang:1.22-alpine`, services/core) | Pass — core, auth, catalogue, contentsource, curriculummap, learner, question all green |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass |
| `go run ./cmd/migrate` against a fresh disposable `sidus-test` | Pass — 20 migrations applied |
| `go run ./cmd/migrate` rerun | Pass — idempotent, 0 migrations applied |
| `go test ./... -run Integration` against `sidus-test` (single clean run) | Pass — catalogue, contentsource, curriculummap, learner, question all green |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev volumes/network untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `python -m pytest -q` (`D:\Sidus-private-content\tools`, synthetic tests only, no apply/network) | Pass — 88 tests |
| `git diff --check` | Pass — clean |
| Secret/certificate/key/environment/private-content/protected-file audit | Pass — no `.pem`/`.key`/real `.env.local-import` staged, no leaked secret, no private-content path referenced beyond `local-dev`/`tools` |

### Handoff

`docs/handoffs/T-0023.md` "Update (T-0024)"

## T-0023 — Local-only HTTPS Core test environment for pending-source import (D-0022)

**Status:** done / released
**Owner:** Claude Code agent
**Implementation commit:** `1b3cad6c3ee8ab956bad702aea09d0440c0b60a3`
**Depends on:** T-0022 (private, not tracked in this repo)

### Goal

Let the private T-0022 pending-source import tool be exercised against a real running Core over
real TLS with real Clerk-gated auth, before anyone runs the documented 489-record `--apply`,
without creating any `content_sources`, approval, question, rubric, node, or attempt row, and
without touching any licensed source material.

### Delivered

- `docker-compose.local-import.yml` (new): isolated Compose project `sidus-local-import` — own
  Postgres (no host port), one-shot `migrate`, `core` (no host port), and a `caddy:2-alpine`
  HTTPS reverse proxy published **only** on `127.0.0.1:443`.
- `infra/local-import/Caddyfile` (new): static `tls <cert> <key>` reverse proxy to `core:8080`,
  no ACME/auto-HTTPS.
- `.env.local-import.example` (new, placeholders only) + `.gitignore` exception mirroring the
  existing `.env.example` pattern; the real `.env.local-import` stays covered by `.env.*`.
- `docs/local-import-test-environment.md` (new): full setup guide — env vars, private dev-CA/cert
  generation under `D:\Sidus-private-content\local-dev` (never this repo), start/stop, health
  check, Clerk sign-in/token retrieval, dry-run, and the documented (not executed) one-record
  smoke-test procedure.
- `docs/local-setup.md`: one pointer paragraph.
- `docs/decisions.md` D-0022: stack design, why the proxy binds `127.0.0.1:443` (keeps T-0022's
  existing exact-HTTPS-origin validator unchanged), why manual openssl CA over mkcert/system
  trust (narrower blast radius).
- Private, not committed (`D:\Sidus-private-content\tools`): `api_client.py` gains an optional,
  additive `ca_bundle_path` parameter (default `None` → byte-for-byte unchanged behavior);
  `register_pending_sources.py` reads it from `SIDUS_CORE_CA_BUNDLE`; new regression-locked tests.

### Constraints

- No production Compose, auth policy, schema, learner, or BFF behavior change.
- No `content_sources`, approval, question, rubric, node, or attempt row created by this task.
- No PDF, book, extracted text, diagram, screenshot, past paper, or mark scheme read or touched.
- The one-record smoke test and the full 489-record `--apply` remain separate, explicit, later
  human actions — not part of this task.

### Open questions / blockers

None.

### Release validation (final pass, 2026-08-08)

| Command | Result |
| --- | --- |
| Git/staged audit (`git status --short`, `git diff --stat`, secret grep) | Pass — only the 9 intended files changed; pre-existing untracked local files untouched; no secret/key material in diff |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` / `-f docker-compose.local-import.yml --env-file .env.local-import.example config` | Pass / Pass / Pass |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run test` (Vitest) | Pass — 289/289, 32 files |
| `npm --prefix apps/web run build` | Pass — 29 routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `gofmt -l . && go build ./... && go vet ./... && go test ./...` (Docker `golang:1.22-alpine`, services/core) | Pass — core, auth, catalogue, contentsource, curriculummap, learner, question all green |
| `docker compose -f docker-compose.test.yml up -d` + `pg_isready` wait | Pass |
| `go run ./cmd/migrate` against a fresh disposable `sidus-test` | Pass — 20 migrations applied |
| `go run ./cmd/migrate` rerun | Pass — idempotent, 0 migrations applied |
| `go test ./... -run Integration` against `sidus-test` (single clean run) | Pass — catalogue, contentsource, curriculummap, learner, question all green |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev volumes/network untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `python -m pytest -q` (`D:\Sidus-private-content\tools`, synthetic tests only, no apply/network) | Pass — 88 tests |
| `git diff --check` | Pass — clean |
| Secret/certificate/key/environment/private-content/protected-file audit | Pass — no `.pem`/`.key`/real `.env.local-import`, no leaked secret, no private-content path referenced beyond `local-dev`/`tools` |

### Handoff

`docs/handoffs/T-0023.md`

## T-0021 — Private licensed-source reference URI (D-0021)

**Status:** done / released
**Owner:** Claude Code agent
**Implementation commits:** `cd6b4964d94c8ea6fd48eaf2e09a9668d3161052`,
`ed67db9098c63ca3e2c02f45b8b43a0c421cf581`
**Depends on:** T-0009 (done, editorial source workflow)

### Goal

Let a pending `content_sources.sourceUrl` accept a tightly validated private reference URI for
licensed 9700 QP/MS pair bundles, without weakening existing HTTP/HTTPS validation, and without
touching any private source file or seeding any runtime data.

### Delivered

- `services/core/internal/contentsource`: one shared strict `sourceUrl` validator
  (`isValidSourceURL`) used identically by `create` and `update` — `create` previously had no
  format check at all; `update`'s separate `isValidHTTPURL` check is now the HTTP/HTTPS half of
  the same function. Both share the existing `invalid_source_url` error code (no new code).
- New accepted form: `sidus-private://licensed/cambridge-international/9700/{session}/{component}`,
  `{session}` exactly `[msw]\d{2}`, `{component}` exactly two digits. The anchored regex rejects
  filesystem paths, drive letters, credentials, query strings, fragments, ports, alternate hosts,
  percent-encoding, case variants, path traversal, backslash-injection, and any other private
  scheme.
- Review fix: `sourceUrl` is now trimmed once, only after validation succeeds, so create/update
  store the canonical trimmed value instead of a whitespace-padded one (including real
  U+00A0 no-break-space, not ASCII-only). All-whitespace values still reject before any store call
  via the pre-existing blank-field gates.
- `packages/shared/src/contracts.ts` documents `sourceUrl` as external URL or approved private
  reference URI (comment only; field stays `string`). `source-form.tsx` adds help text and
  switches the input from `type="url"` to `type="text" inputMode="url"` so no browser native
  URL-input parsing can interfere with the private scheme — Core remains the sole format
  authority.
- D-0021 in `docs/decisions.md`; `docs/editorial-source-workflow.md` and
  `docs/content-provenance-register.md` note the new form and restate the rights-approval gate is
  unchanged.
- Tests: full Go create/update matrix (valid HTTP(S), valid private URI, every rejected variant
  including `localhost`/traversal/backslash), canonical-trim storage regression tests, and a
  blank-after-trim regression test. Web test confirms the hint renders and the private URI is
  submitted to Core verbatim.
- Confirmed by inspection (no code change needed): `services/core/internal/learner` only uses
  `source_url` in a gate predicate — it is never selected into any learner-facing type, so no
  learner surface exposes `sourceUrl` before or after this change.
- No schema/migration change (`content_sources.source_url` has no format `CHECK` constraint).

### Release validation (2026-08-07)

| Check | Result |
| --- | --- |
| Git status / staged-file audit | Pass — only the 11 T-0021 files in the commit range; untracked `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx` remain unstaged |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 32 files, 289 tests |
| Web typecheck / production build | Pass / Pass — all 29 routes intact |
| Strict shared-contract TypeScript (`tsc --noEmit --strict packages/shared/src/contracts.ts`) | Pass |
| Go `gofmt` / `build` / `vet` / unit (Docker `golang:1.22-alpine`) | Pass / Pass / Pass / Pass — all 7 packages |
| Fresh disposable `sidus-test` migrate (0001–0020) / rerun | Pass — 20 applied / 0 applied (idempotent) |
| Full Go integration suite, single run on freshly migrated DB | Pass — 83 tests, catalogue/contentsource/curriculummap/learner/question all green |
| Disposable `sidus-test` teardown | Pass — scoped `down -v`; only `sidus-test` project resources removed |
| Python pytest (`services/ai`) | Pass — 18 tests; pre-existing deprecation warnings only |
| `git diff --check` | Pass — no whitespace errors |
| Secret / protected-file / source-content / env-file audit | Pass — no secret pattern, no forbidden extension, no `D:\Sidus-private-content` access (one matching string is a rejected-URL test fixture), no `.env` file touched |

Note: re-running the integration suite a second time against an already-mutated disposable
database (immutable audit tables cannot be cleaned between runs) produces unrelated failures in
whichever package runs next — this is the same documented cross-run contamination pattern as
T-0018's handoff, not a regression. A single run against a freshly migrated database is the
supported test mode and passed completely.

### Constraints

- Release commit is documentation-only (this history move, handoff status, `CLAUDE.md`). No
  implementation commit amended; no product/Core/AI/BFF/database/route/business-rule change.
- Protected files (`.claude/`, `.claude-flow/`, images, spreadsheets, `.env.local`) remain
  untouched and unstaged. No `D:\Sidus-private-content` file was read, rendered, hashed, copied,
  moved, or uploaded.

### Handoff

`docs/handoffs/T-0021.md`. See D-0021 in `docs/decisions.md`.

## T-0018 — Licensed-adaptation provenance for questions

**Status:** done / released
**Owner:** Codex / Claude Code release
**Implementation commit:** `50ea7b2b0b80250fc4607948ef8f2bff243288f1`
**Depends on:** T-0017 (done / released)

### Goal

Require every newly created question to declare explicit `original` or `licensed_adaptation`
origin, persist immutable metadata-only licensed provenance, and re-check live source rights at
every subsequent question write, verification, and learner-facing read — without ever copying or
inferring source content, and without touching historical question rows.

### Delivered

- Additive, idempotent migration 0020: nullable `questions.origin_type`, immutable
  `question_provenance` (one row per licensed question, DB-trigger-enforced no update/delete),
  append-only names-only `question_provenance_events`, and a DB trigger that blocks any change to
  `origin_type` after create — including on historical NULL rows. No backfill, no seed, no data
  mutation.
- Strict Core create contract requires exact case-sensitive `originType`; `original` rejects any
  source id/locator, `licensed_adaptation` requires a non-blank locator plus an approved,
  rights-complete (`owner`, `title`, `sourceUrl`, `sourceHash`, `licenceReference`,
  `permittedUse`, `allowedAudience`), catalogue-syllabus-linked source. Actor is the authenticated
  Clerk subject, never caller-supplied.
- Licensed gate reruns live (plain SQL, not cached) on every question/rubric write, question
  verification, learner `list`/`get`, attempt creation, and attempt feedback submission — a source
  that regresses (expired, rejected, unlinked, rights-incomplete, wrong syllabus) makes the
  question unavailable everywhere with no latest/fallback source resolution.
- Learner `Projection`, `Attempt`, and `AttemptResult` types carry no origin, source, locator,
  licence, actor, or provenance timestamp field — provenance is structurally unreachable from
  learner routes, not just filtered.
- Editorial create UI adds required origin choice, an approved rights-complete syllabus-linked
  source picker, a metadata-only locator field, immutable post-create display, and an explicit
  rights/content warning. No upload/preview/OCR/AI path added.
- `docs/licensed-adaptation-provenance.md` documents the human licence-evidence approval workflow;
  D-0020 in `docs/decisions.md` records the decision and rejected alternatives.

### Release validation (2026-08-07)

| Check | Result |
| --- | --- |
| Protected working-tree audit | Pass — only `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, `Sidus*.xlsx` untracked; ignored `.env.local`/`Guidelines.pdf` untouched |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 32 files, 288 tests |
| Web typecheck / production build | Pass / Pass — 15 pages generated; all routes intact |
| Strict shared-contract TypeScript | Pass (covered transitively by web typecheck) |
| Go `gofmt` / `build` / `vet` / unit | Pass / Pass / Pass / Pass — all 7 packages |
| Fresh disposable migrate (0001–0020) / rerun | Pass — 20 applied / 0 applied (idempotent) |
| Full Go integration suite | Pass — catalogue, content-source, curriculum-map, learner, question all green, including `ProvenanceCatalogueLinking`, `ProvenanceMigrationRerunNoBackfill`, `LicensedProvenanceImmutableAndNamesOnly`, `LicensedSourceRegressionBlocksVerification`, and `LicensedSourceRegressionExcludesDeliveryAndAttempt` |
| Disposable `sidus-test` teardown | Pass — scoped `down -v`; container/network removed, dev volume untouched |
| Python pytest (`services/ai`) | Pass — 18 tests; pre-existing deprecation warnings only |
| `git diff --check` | Pass |
| Staged-content and secret audit | Pass — 27 files in T-0018 commit, all within allowed scope, no forbidden extension/path/secret pattern |

### Constraints

- Release commit is documentation-only (this history move, handoff status, `CLAUDE.md`). No
  implementation commit amended; no product/Core/AI/BFF/database/route/business-rule change.
- Protected files (`.claude/`, `.claude-flow/`, images, spreadsheets, `.env.local`,
  `Guidelines.pdf`) remain untouched and unstaged.

### Handoff

`docs/handoffs/T-0018.md`. See D-0020 in `docs/decisions.md` and
`docs/licensed-adaptation-provenance.md`.

## T-0017 — Sidus Observatory visual system and responsive frontend polish

**Status:** done / released
**Owner:** Claude Code agent / Codex release
**Implementation commits:** `26a4a8a`, `0b67953`, `e2acb48`, `d35eebc`
**Depends on:** T-0016 (done / released)

### Goal

Apply Sidus Observatory design system across `apps/web` presentation without changing product,
Core, AI, BFF, database, route, dependency, or business-rule behavior.

### Delivered

- Light-first white + blue-ink and dark-mode navy tokens, fallback-stack typography, original
  inline `A*` logo, persisted theme control, and shared presentation primitives.
- Central lifecycle-status and MCQ option-state helpers used across landing, shell/navigation,
  Practice Mode, and all editorial workspaces.
- Accessible non-color-only states, visible focus, reduced motion, responsive 390px behavior,
  roving-tabindex MCQ radiogroup, and single-row scrolling mobile navigation.
- Release fixes add root hydration-warning suppression and replace Skeleton gradient animation
  with token-based opacity pulse respecting reduced motion. No CSS `gradient(` remains.

### Release validation (final pass, 2026-08-06)

| Check | Result |
| --- | --- |
| Protected working-tree audit | Pass — tracked tree/index clean before release docs; only `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, and `Sidus*.xlsx` untracked; ignored `.env.local` untouched |
| Gradient/Skeleton/root HTML invariants | Pass — zero `gradient(` matches; Skeleton tokenized with reduced-motion override; root HTML suppresses expected theme hydration difference |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 32 files, 287 tests |
| Web typecheck / production build | Pass / Pass — 15 pages generated; routes intact |
| Strict shared-contract TypeScript | Pass |
| Go build / vet / unit | Pass / Pass / Pass |
| Fresh disposable migrate / rerun | Pass — 19 applied through 0019 / 0 applied |
| Full Go integration suite | Pass — all catalogue, content-source, curriculum-map, learner, and question integration packages green |
| Disposable `sidus-test` teardown | Pass — scoped `down -v`; container/network removed |
| Python pytest | Pass — 18 tests; dependency/cache warnings only |
| `git diff --check` | Pass |
| Staged-content and secret audit | Pass — protected/content/secret/runtime artifacts excluded |

### Constraints

- Release commit contains documentation only. Protected user files remain untouched and unstaged.
- No product behavior, runtime data, educational content, dependency, migration, or implementation
  commit changed during release.

### Handoff

`docs/handoffs/T-0017.md`. See D-0019 in `docs/decisions.md` and
`docs/sidus-observatory-design-system.md`.

## T-0016 — Practice Mode MCQ attempts, deterministic marking, and verified feedback foundation

**Status:** done / released
**Owner:** Codex
**Priority:** P1
**Implementation commits:** `fddce8ea706cb43980ee35f5bd3ce393791b8bbc`, `f6b1b763902c9b88f95921fc5ec3115bc9b63a1a`
**Depends on:** T-0015 (done / released)

### Goal

Add strict editorial MCQ feedback, owned immutable learner attempts, atomic deterministic submit,
learner-safe BFF contracts, and Practice Mode feedback without widening into Exam Mode or AI.

### Delivered

- Immutable MCQ rubrics require complete, exact verified feedback bound to current option IDs;
  non-MCQ feedback is forbidden, historical rows remain untouched, and no feedback/content is
  seeded.
- Additive migration 0019 creates owner-bound, pinned `open | submitted` attempts and append-only
  names-only events. Attempt creation atomically rechecks learner eligibility and pins exact owned
  canonical rubric plus current question revision.
- Corrupt persisted rubric/option sets return learner-safe not-found before attempt/event mutation.
- Submission locks own open attempt, marks all-or-zero once from pinned verified feedback, returns
  exact learner-safe result fields, and makes replay/concurrent submit conflict. Foreign owners see
  not-found.
- Separate fixed-operation learner BFF write routes enforce exact bodies and a shared 4096-byte
  hard cap. Declared and streamed oversized bodies return 413 before Clerk or Core access.
- Practice UI explicitly labels selected/correct states, renders complete approved feedback only
  after submit, retains pinned attempts for safe retry, and prevents duplicate creation/submission.
- No raw rubric, source data, actor IDs, events, timestamps, canonical IDs, AI, timer, Exam Mode,
  or learner-attempt administration route is exposed.

### Release validation (final pass, 2026-08-06)

| Check | Result |
| --- | --- |
| Protected working-tree audit | Pass — tracked tree/index clean; only `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, and `Sidus*.xlsx` untracked; ignored `.env.local` and `Guidelines.pdf` untouched |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 28 files, 256 tests |
| Web typecheck / production build | Pass / Pass — learner create/submit and Practice routes present |
| Strict shared-contract TypeScript | Pass |
| Go changed-file gofmt / build / vet | Pass / Pass / Pass (Docker `golang:1.22-alpine`) |
| Full Go unit tests | Pass — all packages green |
| Fresh disposable migrate / rerun | Pass — 19 applied through 0019 / 0 applied |
| Full integration suite | Pass — all packages green, including corrupt-data, ownership, pinned lifecycle, and parallel-submit regressions |
| Disposable `sidus-test` teardown | Pass — `down -v`; container/network removed |
| Python pytest | Pass — 18 tests; dependency/cache warnings only |
| T-0016 release invariants | Pass — exact current-option feedback, owned/current canonical pins, atomic one-shot submit, exact projections, closed capped BFF, accessible guarded UI |
| `git diff --check` | Pass |
| Staged content and secret audit | Pass — docs-only release files; protected/content/secret/runtime artifacts excluded |

### Constraints and remaining manual gates

- Release commit contains documentation only. No question/rubric prose, source extract, PDF, ZIP,
  secret, design artifact, runtime data, or `.env.local` is staged or committed.
- Protected user files remain untouched.
- Runtime Clerk/Core configuration and human-authored verified content remain operator/editorial
  gates; release changes no runtime data or product behavior.

### Handoff

`docs/handoffs/T-0016.md`. See D-0018 in `docs/decisions.md`.

## T-0015 — Learner-safe verified-question delivery foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Start commit:** `80e130abac01b999dbb87433b96d26a92cd86e54`
**Implementation commits:** `d60f6e8`, `9ee5561`
**Depends on:** T-0007 (done), T-0013 (done), T-0014 (done)

### Goal

Let every signed-in recognized role discover and read only eligible learner-safe questions, with
no editorial, answer, audit, source, or lifecycle data exposed.

### Delivered

- Dedicated Core learner package and `GET /learner/questions`, `GET
  /learner/questions/{id}`, and `GET /learner/syllabuses` routes protected by
  `learner_question:read`; unknown roles remain denied.
- Explicit learner question and active-syllabus projections. Question responses contain only
  `id`, `syllabusId`, `curriculumMapNodeId`, `responseType`, `language`, `prompt`, `options`, and
  `contentRevision`.
- Read-time eligibility requires verified question, owned verified current canonical rubric,
  verified curriculum node, and approved catalogue-linked source. No latest-rubric fallback.
- Foreign canonical-rubric ownership regression covered and blocked by
  `rv.question_id = q.id`.
- Separate fixed-operation GET-only learner BFF validates identifiers before auth/fetch, refuses
  redirects, and sanitizes upstream 5xx responses.
- `/dashboard/practice` discovers active syllabuses through an accessible select. Unknown role
  makes zero learner BFF calls. MCQ selection remains local-only; no submit, marking, answer
  reveal, attempt/session, timer, Exam Mode, AI, or learner write route exists.

### Release validation (final pass, 2026-08-06)

| Check | Result |
| --- | --- |
| Protected working-tree audit | Pass — tracked tree/index clean; only `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, and `Sidus*.xlsx` untracked |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 27 files, 238 tests |
| Web typecheck / production build | Pass / Pass — learner question, syllabus, and practice routes present |
| Strict shared-contract TypeScript | Pass |
| Go build / vet / changed-file gofmt | Pass / Pass / Pass (Docker `golang:1.22-alpine`) |
| Full Go unit tests | Pass — all packages green |
| Fresh disposable migrate / rerun | Pass — 18 applied through 0018 / 0 applied |
| Full integration suite | Pass — all packages green, including learner ownership and syllabus-discovery regressions |
| Disposable `sidus-test` teardown | Pass — `down -v`; container/network removed |
| Python pytest | Pass — 18 tests; dependency/cache warnings only |
| Learner release invariants | Pass — explicit projections, complete live eligibility gate, closed BFF, role/UI exclusions |
| `git diff --check` | Pass |
| Staged content and secret audit | Pass — docs-only release files; protected/content/secret/runtime artifacts excluded |

### Constraints and remaining manual gates

- Release commit contains documentation only. No question/rubric prose, source extract, PDF,
  ZIP, secret, design artifact, runtime data, or `.env.local` is staged or committed.
- Protected untracked user files remain untouched.
- Runtime Clerk/Core configuration and human-authored eligible content remain operator/editorial
  gates; release changes no runtime data or product behavior.

### Handoff

`docs/handoffs/T-0015.md`. See D-0017 in `docs/decisions.md`.

## T-0014 — Explicit canonical rubric selection

**Status:** done / released
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0007 (done), T-0012 (done), T-0013 (done)

### Goal

Require reviewer/admin selection of one current verified rubric when verifying a question, persist
that rubric as the question's canonical version, and allow explicit repair of historical verified
questions whose canonical reference is null.

### Delivered

- Additive, rerunnable migration 0018 adds nullable question-owned canonical rubric FK and performs
  no backfill.
- Verification requires explicit positive `rubricVersion`; selected rubric must belong to question,
  be verified, and match current question content revision.
- Core atomically writes verified status, canonical rubric ID, and names-only audit event after
  grounding/source-gate recheck. Existing canonical selection cannot be replaced.
- Reviewer/admin-only repair sets historical verified/null canonical selection once; invalid
  lifecycle, foreign, draft, stale, and replacement attempts fail safely.
- Shared contracts, fixed-operation editorial BFF, and role-gated UI expose explicit selection and
  canonical marker without adding learner delivery.

### Release validation (final pass, 2026-08-06)

| Check | Result |
| --- | --- |
| Protected working-tree audit | Pass — tracked clean; only `.claude/`, `.claude-flow/`, `DB.jpeg`, `arch.jpeg`, and `Sidus*.xlsx` untracked |
| Staging/content audit before release | Pass — no staged files before docs update; protected/content/secret/runtime artifacts excluded |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 21 files, 197 tests |
| Web typecheck / production build | Pass / Pass — canonical editorial route present; no learner route |
| Strict shared-contract TypeScript | Pass |
| Go build / vet / changed-file gofmt | Pass / Pass / Pass (Docker `golang:1.22-alpine`) |
| Full Go unit tests | Pass — all packages green |
| Fresh disposable migrate / runner rerun | Pass — 18 applied / 0 applied |
| Canonical selection invariants | Pass — strict body, owned/verified/current rubric, atomic persistence, no replacement, one-time repair |
| Full integration tests | Pass — catalogue, contentsource, curriculummap, question green |
| Disposable `sidus-test` teardown | Pass — `down -v`; container/network removed |
| Python pytest | Pass — 18 tests; dependency/cache warnings only |
| `git diff --check` | Pass |

### Constraints and remaining manual gates

- Release commit contains documentation only. No question/rubric prose, source extracts, PDF,
  secret, design artifact, runtime data, or `.env.local` file is staged or committed.
- Protected untracked user files remain untouched: `.claude/`, `.claude-flow/`, `DB.jpeg`,
  `arch.jpeg`, and `Sidus*.xlsx`.
- Runtime Clerk/Core environment configuration remains operator gate. Human editorial reviewers
  must make every canonical selection. Future learner projection must omit canonical ID, rubric,
  and answer key.

### Handoff

`docs/handoffs/T-0014.md`. See D-0016 in `docs/decisions.md`.

## T-0013 — Deterministic MCQ delivery schema

**Status:** done / released
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0007 (done), T-0012 (done)

### Goal

Add safe question-owned multiple-choice options and rubric-versioned canonical correct-option
support without adding learner delivery or marking behavior.

### Delivered

- Additive, rerunnable migration 0017 adds nullable `questions.options`; existing rows remain valid
  with `NULL`, and no content is seeded.
- Core and shared contracts enforce exact ordered 2–6 stable-ID/original-label options for MCQ and
  prohibit options for non-MCQ questions.
- MCQ rubric versions require `answerKey.correctOptionId`, bound under question row lock to current
  options. Non-MCQ rubrics reject answer keys.
- Option edits increment `content_revision` once, stale older rubrics, and record field names only;
  rejected and no-op writes do not mutate.
- Existing fixed-operation editorial BFF forwards bodies verbatim. Draft UI supports option
  add/remove/reorder and current-option answer-key selection without default content.

### Release validation (final pass, 2026-08-06)

| Check | Result |
| --- | --- |
| Docker daemon | Pass — Docker Desktop client/server 29.4.2 |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 21 files, 181 tests |
| Focused MCQ BFF/UI validation | Pass — 3 files, 22 tests |
| Web typecheck / production build | Pass / Pass — no learner route added |
| Strict shared-contract TypeScript | Pass |
| Go build / vet / changed-file gofmt | Pass / Pass / Pass (Docker `golang:1.22-alpine`) |
| Full Go unit tests | Pass — all packages green |
| Fresh disposable migrate / runner rerun | Pass — 17 applied / 0 applied |
| Direct 0017 rerun | Pass — two direct reruns |
| Options revision / answer-key binding | Pass — unit and PostgreSQL integration coverage green |
| Full integration tests | Pass — catalogue, contentsource, curriculummap, question green |
| Disposable `sidus-test` teardown | Pass — `down -v`; container/network removed |
| Python pytest | Pass — 18 tests; dependency deprecation warnings only |
| `git diff --check` | Pass |

### Constraints and remaining manual gates

- Release commit contains documentation only. No question/rubric/source material, PDF, secret,
  design artifact, runtime data, or `.env.local` file was staged or committed.
- Protected untracked user files remain untouched: `.claude/`, `.claude-flow/`, `DB.jpeg`,
  `arch.jpeg`, and `Sidus*.xlsx`.
- Runtime Clerk/Core environment configuration remains operator gate. Human editorial users must
  author and review content. Any learner delivery must use a separate answer-key-free projection.

### Handoff

`docs/handoffs/T-0013.md`. See D-0015 in `docs/decisions.md`.

## T-0012 — Private editorial question and rubric workflow

**Status:** done / released
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0007 (done), T-0009 (done), T-0010 (done)

### Goal

Add secure same-origin editorial workflow for existing Core question and versioned-rubric
operations. Keep Core authoritative and create or change no runtime content automatically.

### Delivered

- Protected `/dashboard/editorial/questions` workflow for editor/reviewer/admin roles, with denied
  learner/unknown state making zero workflow API calls.
- Fixed-operation question/rubric BFF with strict pre-auth path/query/body validation, server-only
  Clerk token forwarding, generic Core `5xx` handling, and redirect refusal.
- Question and rubric authoring/review UI covering all lifecycle states, revision freshness,
  confirmations, safe domain errors, and empty/loading/retry/mobile states.
- Minimal Core read correction: authorized editorial reads return draft/verified/retired records;
  list accepts validated `draft`/`verified`/`retired`/`all`. Write rules and permissions unchanged.

### Release validation (final pass, 2026-08-05)

| Check | Result |
| --- | --- |
| Docker daemon | Pass — client/server 29.4.2 |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 21 files, 176 tests |
| Focused question BFF policy | Pass — 2 files, 72 tests |
| Web typecheck / production build | Pass / Pass — 22 dynamic routes |
| Strict shared-contract TypeScript | Pass |
| Go build / vet / changed-file gofmt | Pass / Pass / Pass (Docker `golang:1.22-alpine`) |
| Full Go unit tests | Pass — all packages green |
| Core question lifecycle/filter/role matrix | Pass — unit and integration coverage green |
| Fresh disposable migrate / rerun | Pass — 16 applied / 0 applied |
| Full integration tests | Pass — catalogue, contentsource, curriculummap, question green |
| Disposable `sidus-test` teardown | Pass — `down -v`; container/network removed |
| Python pytest | Pass — 18 tests |
| `git diff --check` | Pass |

### Constraints and remaining manual gates

- No question/rubric prose, source material, PDF, secret, design artifact, runtime data, or
  `.env.local` file was staged or committed.
- Protected untracked user files remain untouched: `.claude/`, `.claude-flow/`, `DB.jpeg`,
  `arch.jpeg`, and `Sidus*.xlsx`.
- Runtime Clerk/Core environment configuration remains operator gate. Human editorial users must
  explicitly author and review any question or rubric content.

### Handoff

`docs/handoffs/T-0012.md`. See D-0014 in `docs/decisions.md`.

## T-0011 — Biology vertical-slice scope realignment

**Status:** done / released
**Owner:** Codex
**Priority:** P1
**Depends on:** T-0004 (superseded), T-0005 (done)

### Goal

Replace Cambridge O Level Biology 5090 in active product scope with one combined,
metadata-only Cambridge International AS & A Level Biology 9700 catalogue row. Preserve 5090
catalogue identity, history, provenance, and downstream records.

### Delivered

- Additive migration 0016 makes 0610 Extended and exactly one combined 9700 row active.
- Existing 5090 catalogue row remains present as retired historical metadata; no deletion or
  downstream mutation.
- Migration conflict path uses `DO NOTHING`, preserving human-edited existing 9700 data on direct
  historical SQL rerun.
- Integration coverage proves fresh scope, active resolution, retained retirement history, zero
  seeded 9700 content, migration idempotence, and direct-rerun preservation.
- Project scope, catalogue, curriculum, provenance, question-model, setup, and decision docs align
  with D-0013.

### Release validation (final pass, 2026-08-05)

| Check | Result |
| --- | --- |
| Docker daemon | Pass — Docker Desktop 29.4.2 server reachable and healthy |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 16 files, 123 tests |
| Web typecheck / production build | Pass / Pass |
| Strict shared-contract TypeScript | Pass |
| Go build / vet | Pass / Pass (Docker `golang:1.22-alpine`) |
| Changed-file gofmt | Pass — no changed Go file listed |
| Full Go unit tests | Pass — all packages green |
| Fresh disposable migrate / rerun | Pass — 16 applied / 0 applied |
| Fresh Biology scope | Pass — 0610 + 9700 active; 5090 retained retired |
| Fresh 9700 content absence | Pass — zero sources, nodes, questions, rubrics |
| Direct 0016 human-edit preservation | Pass |
| Full integration tests | Pass — catalogue, contentsource, curriculummap, question green |
| Disposable `sidus-test` teardown | Pass — `down -v`; container/network removed |
| Python pytest | Pass — 18 tests |
| `git diff --check` | Pass |

### Constraints and remaining manual gates

- No PDF read/import, source/node/question/rubric content, secrets, design bundle artifact, or
  `.env.local` file was staged or committed.
- Protected untracked user files remain untouched: `.claude/`, `.claude-flow/`, `DB.jpeg`,
  `arch.jpeg`, `Sidus*.xlsx`; ignored local PDF and `.env.local` files also remain untouched.
- Human editor/admin must register any future 9700 source, verify rights/provenance, approve and
  catalogue-link it, then author and review downstream metadata. Runtime Clerk/Core configuration
  remains operator gate.

### Handoff

`docs/handoffs/T-0011.md`. See D-0013 in `docs/decisions.md`.

## T-0010 — Private editorial curriculum-map workflow

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0006 (done), T-0009 (done)

### Goal

Extend the T-0009 secure Next.js editorial workflow so staff can author, edit, review, and
retire curriculum-map nodes through the browser. Core remains the sole authority for rights,
source, syllabus, parent, cycle, and lifecycle rules. No node/source data is created, seeded,
verified, retired, or altered automatically by this task.

### Delivered

- Protected `/dashboard/editorial/curriculum` workflow for editors, reviewers, and admins;
  learner/unknown roles see an access-denied state and trigger zero API calls.
- Closed-union BFF routes for list/get/create/update/verify/retire curriculum-map operations;
  fixed method/path allowlist, pre-auth query/id validation, sanitized Core `5xx` handling,
  and redirect failure remain fail-closed.
- Metadata-only authoring UI with loading, empty, error/retry, create, edit, verify-confirm,
  retire-confirm, disabled, role-gated, and mobile-safe states.
- User-approved Core read-contract widening: authorized staff reads can return any node
  lifecycle status; list supports `draft`, `verified`, `retired`, and `all`, with omitted status
  equivalent to `all`. No schema, migration, permission, or write-side rule changed.
- Review findings fixed in `9a7cd02`: explicit `status=all`, stable invalid-status handling,
  and empty/overlong/unsafe `syllabusId` rejection before auth or Core fetch.

### Acceptance checks

- All BFF route exports and fixed Core operation mappings verified. — met
- Empty, overlong, and unsafe `syllabusId` plus invalid status rejected before auth/fetch. — met
- Draft/verified/retired/all and omitted-status Core behavior verified. — met
- Core `5xx` responses sanitized and redirects fail closed. — met
- Web Vitest, typecheck, production build, and strict shared-contract TypeScript pass. — met
- Dockerized Go build, vet, changed-file gofmt, unit tests, migrations, and integration tests
  pass. — met
- Python pytest and `git diff --check` pass. — met
- No protected user file or prohibited content/artifact was staged. — met

### Release validation (final pass, 2026-08-05)

| Check | Result |
| --- | --- |
| Docker daemon | Pass — Docker Desktop 29.4.2 server reachable and healthy |
| Dev/test Compose config | Pass / Pass |
| Web Vitest | Pass — 16 files, 123 tests |
| Targeted BFF guard suite | Pass — 5 files, 59 tests |
| Web typecheck / production build | Pass / Pass — 16 routes registered |
| Strict shared-contract TypeScript | Pass |
| Go build / vet | Pass / Pass (Docker `golang:1.22-alpine`) |
| Changed-file gofmt | Pass — no changed Go file listed |
| Full Go unit tests | Pass — all packages green |
| Fresh disposable migrate / rerun | Pass — 15 applied / 0 applied |
| Full integration tests | Pass — catalogue, contentsource, curriculummap, question green |
| Disposable `sidus-test` teardown | Pass — `down -v`; stack absent afterward |
| Python pytest | Pass — 18 tests |
| `git diff --check` | Pass |

### Constraints and remaining manual gates

- No content ingestion, OCR, AI generation, copyrighted text, PDFs, diagrams, questions,
  rubrics, source/node content, secrets, design ZIPs, or `.env.local` files were committed.
- Protected untracked user files remain untouched: `.claude/`, `.claude-flow/`, `DB.jpeg`,
  `arch.jpeg`, `Sidus*.xlsx`, and both ignored `.env.local` files.
- Human editor/admin must still approve and catalogue-link eligible sources, then explicitly
  author/review curriculum-map metadata. Runtime Clerk/Core environment configuration remains
  an operator gate.

### Handoff

`docs/handoffs/T-0010.md`. See `D-0008` "Update (T-0010)" and `D-0012` in
`docs/decisions.md`.

## T-0009 — Private editorial source workflow

**Status:** done
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0003 (done), T-0004 (done), T-0005 (done), T-0008 (done)

### Goal

First secure web-to-Core workflow for editorial staff: source registry → metadata completion →
catalogue syllabus association → rights review → approval/rejection. This unlocks human
approval/linking for the seeded 0610/5090 sources (T-0001/T-0005) — it does not perform those
approvals or links itself.

### Scope

- Protected `/dashboard/editorial/sources` page in `apps/web`, gated to `editor`/`reviewer`/
  `admin` (server-side role check from the verified Clerk session; `learner`/unknown see no
  editorial controls).
- Narrowly scoped Next.js route handlers (BFF) that proxy only allowlisted Core endpoints:
  `GET/POST /content-sources`, `GET/PATCH /content-sources/{id}`,
  `POST /content-sources/{id}/approve`, `POST /content-sources/{id}/reject`,
  `GET /catalogue/syllabuses`. No open proxy — fixed Core base URL from server-only
  `SIDUS_CORE_API_URL`, explicit per-route operation, no caller-controlled target URL.
- Fail closed: missing `SIDUS_CORE_API_URL` or missing Clerk session token → safe
  unavailable/unauthorized JSON response, never a leaked secret or raw upstream error. Core
  `5xx` responses and redirects are sanitized to the same generic failure (T-0009 review fix,
  D-0011 update).
- Core stays the sole authorization authority — the web role check only hides/shows UI
  controls; every mutation is still enforced (401/403) by Core's existing `auth.Protect`.
- No curriculum-map or question/rubric authoring UI (out of scope).
- No content ingestion, OCR, AI generation, or copyrighted material of any kind.

### Assumptions

- `SIDUS_CORE_API_URL` is a new server-only env var (never `NEXT_PUBLIC_*`), analogous to how
  Core already gates on `DATABASE_URL`/Clerk config (`services/core/main.go`).
- The web app had no existing test runner; Vitest + Testing Library was added (dev deps only,
  package-lock changes expected and in scope for this task).
- `packages/shared` was not yet wired into `apps/web`'s module resolution; a `tsconfig.json`
  `paths` entry was added so it resolves without npm workspaces (no root `package.json`
  exists).
- UI role visibility reads the verified `sidus_role` Clerk session claim server-side
  (`sessionClaims`, populated by Clerk from the signed token — not client-suppliable); this is
  cosmetic only, per D-0006/D-0009 precedent that Core is the sole authorization authority.

### Open questions

None blocking. Manual human approval/linking of the seeded 0610/5090 sources through this new
UI is intentionally left to a human editor/admin after this task lands (matches T-0005's
existing carried-forward note).

### Handoff

`docs/handoffs/T-0009.md`. See `D-0011` in `docs/decisions.md` for the architecture decision
and its "Update (T-0009 review)" note for the Core-5xx/redirect sanitization fix.

## T-0001 — Content rights/provenance gate

**Status:** done
**Owner:** Claude Code agent
**Priority:** P0
**Scope:** Add local PostgreSQL, rights/provenance schema, immutable reviews, shared contracts, core API checks, AI ingestion rejection, official-syllabus metadata seeds, tests, setup docs.

### Acceptance checks

- Docker Compose starts PostgreSQL. — met
- Empty database migration succeeds. — met
- Source states: `pending`, `approved`, `rejected`, `expired`. — met
- Approval requires owner, source URL, source hash, licence reference, permitted use, allowed audience, reviewer, decision date. — met
- AI ingestion rejects every non-approved source with auditable reason. — met
- Only source metadata for official 0610/5090 links is seeded. — met
- No source PDFs, extracts, diagrams, or derivative questions added. — met
- Relevant tests pass. — met, see handoff for exact results

### Constraints

- Existing untracked root images/spreadsheets are user files. Never staged, altered, moved, or deleted.
- No Redis in this task.
- No auth UI in this task.

### Open questions (carried forward, not blocking)

- Review authorization model: temporary internal reviewer identifier used now (`reviewer_id` required string); no auth system assumed. Revisit when full auth task lands.
- Seed rows for CAM-0610-2026 / CAM-5090-2026 leave `owner`, `source_hash`, `licence_reference`, `allowed_audience` null — provenance register does not document these. Seeds stay `pending` and cannot pass approval until a human reviewer supplies those fields.
- No update/edit endpoint is in scope (Core API list is create/get/list/approve/reject only). Rights fields must be supplied at creation time or a source stays blocked from approval permanently. Follow-up task needed for a `PATCH /content-sources/{id}` endpoint.
- `expired` is a valid `status` value in the schema/contracts for forward-compatibility but no endpoint transitions a source to `expired` in this task.

### Handoff

`docs/handoffs/T-0001.md`

## T-0002 — Pending source metadata update

**Status:** done
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done)

### Goal

Let human curators complete metadata for `pending` content sources (including the seeded
0610/5090 syllabus rows) without bypassing rights approval. Add an auditable, append-only
event trail for every successful update.

### Scope

- `PATCH /content-sources/{id}` on Core API.
- Only `pending` sources may be updated; `approved`/`rejected`/`expired` return `409`.
- Updatable fields: `title`, `owner`, `sourceUrl`, `sourceHash`, `licenceReference`,
  `permittedUse`, `allowedAudience`, `syllabusCode`.
- Reject empty/whitespace-only values for any supplied field (`400`).
- `syllabusCode` must be `0610` or `5090` (`400`).
- `sourceUrl` must be an absolute HTTP/HTTPS URL (`400`).
- Duplicate `sourceUrl` returns `409`.
- Update `updated_at`.
- `actorId` required in request body (`400` if missing).

### Audit

- New append-only `content_source_events` table. `content_source_reviews` untouched.
- Every successful update records: source ID, `event_type = 'metadata_updated'`, actor ID,
  event time, changed field **names** only.
- Never store PDF content, extracted text, diagrams, or previous/new field **values**.
- Immutability enforced at DB level (trigger rejects UPDATE/DELETE).

### Rights rule

- PATCH never approves. Approval stays separate and still requires all existing approval
  fields. Seeded syllabus rows stay `pending` until a human supplies verified rights
  metadata and separately approves them. No invented owner/hash/licence/audience data.

### Shared contracts

- `packages/shared`: add PATCH request + source-event contracts. Align Go/Python where relevant.

### Assumptions / decisions

- **Superseded by review finding 1:** changed field names are now a real value-diff.
  `Update` fetches the current row, compares each supplied field against its stored value,
  and only applies/records fields that actually differ. A request where every supplied
  field matches the current value returns `400 no_changes` (no write, no event, `updated_at`
  untouched). A request with no updatable field supplied at all still returns
  `400 no_updatable_fields` (unchanged).
- Updatable fields use pointer/optional JSON so absent vs. present-null both mean "no
  change"; a present field set to `""`/whitespace is a validation error, not a clear.
- `actorId` is a free-text identifier (no auth system yet — mirrors T-0001 `reviewerId`).
- **Review finding 2:** integration tests against `content_source_events` /
  `content_source_reviews` can never clean up (both immutable at the DB level). Removed the
  silent-failure DELETE cleanup attempts; added `docker-compose.test.yml` (disposable
  `postgres-test` service, tmpfs-backed, separate from the dev `postgres` service/volume) and
  documented that `TEST_DATABASE_URL` must point at it, never at dev/prod.
- **Test-environment isolation fix.** `docker-compose.test.yml` had no explicit Compose
  project name, so it defaulted to the same project (`sidus`) as `docker-compose.yml`,
  risking `down -v` on the test file removing dev resources. Fixed: added `name: sidus-test`
  (and `name: sidus` to the dev file) so containers/networks/volumes are fully distinct;
  verified by running both stacks simultaneously and tearing down only the test one. Also
  corrected the file's header comment, which referenced a nonexistent Compose `migrate`
  service — migrations run via `go run ./cmd/migrate` through the `golang:1.22-alpine`
  image. See `docs/handoffs/T-0002.md` for exact commands.

### Acceptance checks

- Pending source update succeeds. — met (`TestUpdate_Success`)
- Whitespace-only supplied value returns `400`. — met (`TestUpdate_WhitespaceOnlyValue_Returns400`)
- Invalid syllabus code returns `400`. — met (`TestUpdate_InvalidSyllabusCode_Returns400`)
- Invalid/non-HTTP URL returns `400`. — met (`TestUpdate_InvalidSourceURL_Returns400`)
- Duplicate URL returns `409`. — met (`TestUpdate_DuplicateURL_Returns409`)
- Non-pending source update returns `409`. — met (`TestUpdate_NonPending_Returns409`)
- Missing actor ID returns `400`. — met (`TestUpdate_MissingActorID_Returns400`)
- Successful update creates an immutable event, listing only actually-changed fields. — met
  (`TestUpdate_CreatesImmutableEvent`, `TestUpdate_MixedSameAndNewValues_RecordsOnlyChangedFields`,
  live `TestPostgresStore_Integration_UpdateOnlyChangedFields`)
- All-same-value update returns `400 no_changes`, no event, `updated_at` unchanged. — met
  (`TestUpdate_AllSameValues_Returns400NoChanges`, `TestUpdate_NoChangeRequest_NoEventAndNoUpdatedAtChange`,
  live `TestPostgresStore_Integration_UpdateOnlyChangedFields`)
- Existing T-0001 tests remain green. — met
- Live PostgreSQL event-immutability integration test passes against a disposable DB. — met
  (`-run Integration`, all 3 integration tests pass against `docker-compose.test.yml`, DB
  destroyed after)

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `.claude/`, `.claude-flow/`.
- No Redis, auth UI, Exam Mode, or unrelated feature work.

### Open questions (carried forward, not blocking)

- Actor authorization model still deferred to a future auth task (carried from T-0001).

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `docker compose -f docker-compose.yml config` | Pass |
| `docker compose -f docker-compose.test.yml config` | Pass |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./... -v` (unit, Docker toolchain) | Pass — 28 tests pass, 3 integration skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `postgres-test` healthy |
| `go run ./cmd/migrate` against isolated `postgres-test` | Pass — 4 migrations applied |
| `go test ./internal/contentsource/... -run Integration -v` against `postgres-test` | Pass — all 3 integration tests pass |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 5 tests |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass — no type errors |
| `git diff --check` | Pass — no whitespace errors |

### Handoff

`docs/handoffs/T-0002.md`

## T-0003 — Clerk authentication and roles foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done), T-0002 (done)

### Goal

Clerk owns authentication; Sidus Core owns authorization. No user-controlled `actorId` or
`reviewerId`. Audit identity (event actor, review reviewer) derives only from the verified
Clerk session subject.

### Scope

**Web (`apps/web`, Next.js 16 / React 19):** `@clerk/nextjs@^7.6.0`, `ClerkProvider`,
`proxy.ts` middleware protecting `/dashboard(.*)`, Clerk sign-in/sign-up routes, protected
dashboard placeholder, signed-in/out home page.

**Core (`services/core`, Go):** `internal/auth` package (role/permission matrix,
`ParseRole` deny-by-default, `Protect` middleware — 401 missing/invalid token, 403 valid
token lacking permission) backed by the official `clerk-sdk-go/v2` (pinned v2.5.0 for Go
1.22 compatibility) with JWKS TTL caching (no Backend API call per request). Content-source
routes wrapped with required permissions; `actorId`/`reviewerId` removed from request
bodies — audit actor/reviewer come only from the verified subject. Routes mount only when
DB + Clerk are fully configured (fail closed).

**AI (`services/ai`, FastAPI):** `ClerkAuthenticator` (PyJWT `PyJWKClient`, RS256) +
`require_clerk_session` dependency; protected `/ingestion/status` foundation only — no
OCR/ingestion added; rights gate unchanged.

**Contracts/docs:** `packages/shared/src/contracts.ts` — actor/reviewer fields removed,
`SIDUS_ROLES`/`SidusRole`/`SIDUS_ROLE_CLAIM` added. New `docs/auth-setup.md`. `.env.example`
Clerk placeholders only. `docs/decisions.md` D-0006.

### Review follow-up (fail-open hardening)

Closed four fail-open gaps, all now fail closed:

- Core issuer mandatory — content-source routes do not mount without `CLERK_JWT_ISSUER`.
- Authorized parties never silently unrestricted — absent → dev-default local origin only;
  present-but-blank → invalid (Core routes unmounted; AI protected routes → 503).
- AI issuer mandatory — a configured JWKS URL cannot bypass issuer validation; unconfigured
  auth fails closed with a generic 503.
- Content-source bodies parsed strictly (`DisallowUnknownFields` + reject trailing JSON
  values after the first decoded value) — unknown fields (incl. legacy
  `actorId`/`reviewerId`) or a second concatenated JSON value return `400 invalid_json`;
  audit actor/reviewer stay the verified `sub` only.

### Acceptance checks

- Clerk authenticates; Core/AI verify JWT offline via JWKS, no per-request Backend API
  call. — met
- Role authorization from verified `sidus_role` claim; missing/unknown role denied. — met
- `401` missing/invalid token, `403` valid token lacking permission. — met
- `actorId`/`reviewerId` cannot be supplied in request bodies; audit actor/reviewer =
  verified subject. — met
- Content-source routes fail closed without full Clerk/DB configuration. — met
- No real Clerk keys committed/staged; `.env.example` placeholders only. — met
- Relevant tests pass. — met, see release validation below

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `.claude/`, `.claude-flow/`, root `.env.local`.
- No Clerk Dashboard actions performed by the agent (manual human steps below).

### Open questions / blockers

- None blocking. Manual Clerk Dashboard steps (human, before beta): create the Clerk app
  and store real keys only in gitignored `.env.local` files; add session claim
  `sidus_role`; manually set the first admin (`public_metadata.sidus_role = "admin"`);
  configure production domain/origins and set real `CLERK_JWT_ISSUER` /
  `CLERK_AUTHORIZED_PARTIES` / `CLERK_JWKS_URL`. Full detail in `docs/auth-setup.md`.

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; `/api/health` present |
| `go build ./... && go vet ./... && go test ./... -v` (Docker `golang:1.22-alpine`) | Pass — all unit tests green, 3 integration tests skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.test.yml config` / `-f docker-compose.yml config` | Pass / Pass |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `postgres-test` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` postgres | Pass — 4 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — 3 immutable-audit integration tests |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Clean |

### Handoff

`docs/handoffs/T-0003.md`

## T-0004 — Multi-subject syllabus catalogue foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P0
**Depends on:** T-0001 (done), T-0002 (done), T-0003 (done)

### Goal

Build safe, metadata-only multi-subject curriculum catalogue infrastructure so future
approved Cambridge syllabuses can be onboarded without code changes. Remove the hard-coded
`0610`/`5090` two-code restriction from Core validation and shared contracts, replacing it
with registry-backed validation. No content, questions, OCR, source ingestion, or public
copyrighted data is created.

### Scope

- New normalized, metadata-only tables `subjects` and `syllabuses` (+ immutable
  `syllabus_events` audit trail).
- Nullable FK path from `content_sources` to `syllabuses` (`catalogue_syllabus_id`), added
  non-destructively; existing audit tables and rows untouched.
- Registry-backed content-source syllabus validation (resolves a supplied code to a
  registered **active** catalogue syllabus; unknown/inactive/ambiguous → stable 400 before
  DB write; omitted → allowed).
- Authenticated Core catalogue endpoints: readers list/get active syllabuses & subjects;
  admin-only create/update. New least-privilege permissions `content_catalogue:read` and
  `content_catalogue:manage`; deny-by-default for unknown roles.
- Catalogue mutations audited with the verified Clerk subject, names-only (non-content)
  audit trail.
- Shared TypeScript contracts: `SyllabusCode` union → `string`; add catalogue types.
- Docs: D-0007, `docs/curriculum-catalogue.md`, local setup, handoff.

### Schema decisions

- `subjects`: `id` (UUID), `name` (TEXT, UNIQUE), timestamps. Normalized so board/subject
  are never free-text on requests.
- `syllabuses`: `id` (UUID, stable), `board`, `syllabus_code`, `subject_id` (FK → subjects),
  `qualification` (level, e.g. "Cambridge IGCSE" / "Cambridge O Level"), `track` (nullable,
  e.g. "Extended"), `display_name`, `curriculum_year` (nullable — only when explicitly
  known), `status` (`draft`|`active`|`retired`, default `draft`), timestamps.
- Uniqueness: **not** syllabus_code alone. `UNIQUE (board, syllabus_code, COALESCE(track,''))`
  so a NULL track cannot silently duplicate a `(board, code)` pair, and different boards may
  reuse a code.
- FK path: `content_sources.catalogue_syllabus_id UUID NULL REFERENCES syllabuses(id)`, added
  with `ADD COLUMN IF NOT EXISTS` (no data loss). Legacy `syllabus_code` TEXT column kept for
  the two seeded rows; its hard-coded `CHECK (... IN ('0610','5090'))` is dropped so the
  registry — not a fixed enum — is the authority.

### Migration path (backward compatibility)

- Existing seeded content_sources rows (migration 0003) keep their `syllabus_code` text and
  get `catalogue_syllabus_id = NULL`. They are **not** silently auto-mapped to catalogue
  syllabuses; a human links them in a later task. Documented in `docs/curriculum-catalogue.md`.
- New/updated content sources that supply a code have it resolved to an active catalogue
  syllabus, and the resolved `catalogue_syllabus_id` FK is stored alongside the code.

### Seed limits

Seeded exactly:
- Cambridge International / Biology / Cambridge IGCSE / Extended / 0610
- Cambridge International / Biology / Cambridge O Level / (no track) / 5090

Both seeded `active` (they are the D-0004 first vertical slice). Subject `Biology` seeded.
No invented year/edition, objective, assessment rule, rights claim, or extra syllabus.

### Assumptions

- "Readers" for the catalogue = roles that already hold content-source read (editor,
  reviewer, admin). Learner and unknown roles are denied, matching the existing
  content-source matrix and the T-0004 test requirement "learner/unknown denied". A
  public/learner-facing subject picker is explicitly out of scope.
- Seeded biology syllabuses are `active` (not `draft`) because they are the confirmed first
  slice and must resolve for existing 0610/5090 content-source metadata.
- Syllabus association on a content-source request is carried by the existing `syllabusCode`
  field, resolved server-side against the registry. Requests never carry free-text
  board/subject/level. Code resolution requires exactly one active match; zero or multiple →
  treated as unknown (stable 400).
- `curriculum_year` for the two seeds is left NULL: the provenance register documents
  "2026–2028" exam years for the source PDFs, but the task forbids inferring a curriculum
  year/edition into the catalogue without explicit human confirmation.

### Review-fix pass

- Fixed: subject creation now writes an immutable `subject_events` audit row (migration 0009,
  `subject_id`/`event_type`/`actor_id`/`event_time`/`changed_fields`), same transaction as the
  subject insert, append-only via trigger. Migration-seeded Biology subject is documented as an
  explicit bootstrap exception (no event, no invented actor).
- Fixed: catalogue HTTP handlers no longer return raw `err.Error()` for infrastructure failures;
  all map to one stable `internal_error` message. Safe domain errors unchanged.

### Acceptance checks

- Catalogue tables/audit trail created; content-source FK added non-destructively. — met
- Registry-backed syllabus validation replaces hard-coded 0610/5090 enum. — met
- Least-privilege catalogue read/manage permissions; learner/unknown denied. — met
- Catalogue mutations audited with verified Clerk subject, names-only fields. — met
- Only the two D-0004 biology syllabuses seeded, both active; no invented curriculum data. — met
- Internal errors never leak raw infrastructure text. — met
- Relevant tests pass. — met, see release validation below

### Constraints

- Never stage/alter/move/delete untracked user files: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, source ingestion, or copyrighted data. Catalogue holds
  curriculum metadata only.
- No web UI / subject picker. No AI catalogue authority (Core is the single authority).

### Open questions / blockers

- None blocking. Linking the two seeded `pending` content_sources rows to catalogue
  syllabuses is deferred to a future task (requires human provenance confirmation).

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `git diff --check` | Pass — clean |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./... -v` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource green; 9 integration tests skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `postgres-test` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 9 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — 6 catalogue + 3 content-source integration tests |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |

### Handoff

`docs/handoffs/T-0004.md`

## T-0005 — Provenance-confirmed catalogue linking

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0001 (done), T-0002 (done), T-0003 (done), T-0004 (done)

### Goal

Let an editor/admin explicitly confirm and establish a missing or stale
`content_sources.catalogue_syllabus_id` link through the existing authenticated `PATCH
/content-sources/{id}` flow. Migration 0008 (T-0004) intentionally left every existing pending
source's catalogue FK `NULL` rather than auto-mapping it; the pre-T-0005 `syllabusCode` diff
logic compared only the free-text code, so re-supplying the already-stored code was always `400
no_changes` — no safe application-layer path existed to complete the link, and no direct
database workaround is acceptable (bypasses auth/audit).

### Scope

- `services/core/internal/contentsource/postgres_store.go`: `Update` now diffs `syllabusCode`
  text and `catalogue_syllabus_id` independently instead of treating the FK as a byproduct of a
  text-code change.
  - Different code → `syllabus_code` + `catalogue_syllabus_id` updated, audited
    `["syllabusCode"]`.
  - Same code, missing/stale FK → `catalogue_syllabus_id` updated alone, audited
    `["catalogueSyllabusId"]` (never claims `syllabusCode` changed).
  - Same code, matching FK → unchanged `400 no_changes`, no write, no event.
- `handlers_test.go`'s `memoryStore.Update` mirrors the same split-diff logic so handler-level
  tests exercise identical semantics without a live database.
- Request/response shape unchanged: caller still supplies only `syllabusCode`; Core resolves the
  active catalogue syllabus server-side (unchanged registry-backed validation from T-0004).
  Unknown/inactive/ambiguous code still fails `400 unknown_syllabus` before any write.
- No new endpoint, no caller-supplied catalogue ID, no auto-linking (migration/startup/read/
  create/background job), no approval/OCR/ingestion/rights/status change.
- Docs: new `docs/provenance-catalogue-linking.md`; D-0007 updated in place (see
  `docs/decisions.md`); `docs/curriculum-catalogue.md` migration-path section cross-referenced.

### Acceptance checks

- Existing code + `NULL` FK → same-code PATCH links; verified actor; `changed_fields` only
  `catalogueSyllabusId`. — met
- Existing code + wrong FK → safe relink. — met
- Existing code + matching FK → `no_changes`; no event; `updated_at` unchanged. — met
- Different code → code/FK update, correct audit (`["syllabusCode"]`). — met
- Unknown/inactive/ambiguous code → `400` before write; no event. — met
- Omitted code unchanged. — met
- Non-pending source → `409`; no event. — met
- Learner/unknown denied. — met
- Disposable `sidus-test` integration proves FK/audit/immutability. — met
- All existing tests remain green. — met

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, ingestion, or copyrighted data. No rights/status change.
- No silent auto-mapping of the two seeded 0610/5090 content-source rows — a human still calls
  the API per source (documented in `docs/provenance-catalogue-linking.md`).

### Open questions / blockers

- None blocking. The two seeded pending content-source rows remain unlinked
  (`catalogueSyllabusId: null`) until a human editor/admin calls the documented `PATCH` for each
  — this task added the safe path; it does not itself perform the linking. See "Manual steps
  for the two seeded sources" in `docs/provenance-catalogue-linking.md`.

### Release validation (final pass)

| Command | Result |
| --- | --- |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./... -v` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource green; 10 integration tests skipped (no `TEST_DATABASE_URL`) |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `docker compose -f docker-compose.test.yml up -d` + health wait | Pass — `sidus-test-postgres-test-1` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 9 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — 6 catalogue + 4 content-source integration tests, incl. `TestPostgresStore_Integration_ProvenanceCatalogueLinking` |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Pass — clean |

### Handoff

`docs/handoffs/T-0005.md`

## T-0006 — Curriculum-map foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0001 (done), T-0004 (done), T-0005 (done)

### Goal

Build metadata-only curriculum-map infrastructure for all subjects: topic maps, learning
objectives, practical skills, and assessment rules, scoped under the existing curriculum
catalogue. No syllabus text, objective wording, topic labels, assessment text, questions, mark
schemes, PDFs, extracts, diagrams, OCR output, or derivative content is created. No map data is
seeded.

### Scope

- New tables `curriculum_map_nodes` and `curriculum_map_events` (migrations 0010–0011),
  package `services/core/internal/curriculummap`.
- Node fields: stable id, syllabus FK, optional parent-node FK (same syllabus, no cycles),
  `nodeKind` (topic/objective/practical_skill/assessment_rule), unique-per-syllabus
  `nodeCode`, editorial `label` placeholder, lifecycle `status`
  (draft/verified/retired), required approved content-source FK, optional `sourceLocator`,
  timestamps.
- Server-side source gate (Core is sole authority): every create, and every update that
  changes `contentSourceId`, verifies the referenced `content_sources` row exists, is
  `approved`, and its `catalogue_syllabus_id` matches the node's syllabus. Unknown/
  unapproved/unlinked/mismatched → stable `400` before any write.
- New least-privilege permissions `curriculum_map:read` (editor/reviewer/admin, verified-only
  reads), `curriculum_map:create` (editor/reviewer/admin, draft create/PATCH),
  `curriculum_map:verify` (reviewer/admin, verify/retire). Learner/unknown denied.
- Core API: list/get verified nodes by syllabus, create draft, PATCH draft, verify, retire.
  Strict JSON (no caller actor field); every route Clerk-protected; no raw internal error text.
- Shared TypeScript contracts, D-0008, `docs/curriculum-map.md`, `docs/local-setup.md` update.
- No AI-service map authority; AI service untouched.

### Review findings (fixed on top of `b1677cb`, commit `a05c523`)

Four findings from independent review, all fixed in this task's scope. No change to the
authority model, schema purpose, role matrix, approval/provenance rules, public content
restrictions, Clerk setup, or the "no map data seeded" stance. See D-0008 "Update (T-0006
review)".

1. **PATCH strict-JSON bypass** — unknown fields including `actorId`/`reviewerId`/`syllabusId`/
   `status` silently accepted via a `map[string]json.RawMessage` decode where
   `DisallowUnknownFields` has no effect. Fixed with an explicit six-field allowlist plus a
   strict struct decode.
2. **Source gate only ran when `contentSourceId` changed** — now re-validated on every node
   write (update/verify/retire), before any mutation.
3. **`GET /curriculum-map/nodes` returned an empty `200` for an unknown/inactive syllabus** —
   now `400 unknown_syllabus`.
4. **Ancestor walk was not actually row-locked** — every traversed ancestor is now locked `FOR
   UPDATE` in the writing transaction.

### Schema decisions

- Parent-same-syllabus and no-cycle enforcement are done at the **application layer** (Go, same
  transaction, row-locked ancestor walk) rather than a DB trigger — keeps `invalid_parent` error
  mapping under Core's control instead of parsing trigger-raised text. See D-0008
  "Alternatives".
- `syllabusId` is immutable after node creation (not PATCHable).
- `nodeCode` uniqueness is a DB unique index (`syllabus_id`, `node_code`); required
  `content_source_id` is a `NOT NULL` FK.
- `curriculum_map_events` mirrors `syllabus_events`/`content_source_events`: append-only,
  `BEFORE UPDATE OR DELETE` trigger, verified-Clerk-subject actor, changed-field-names only.

### Acceptance checks

- Metadata-only tables + immutable audit trail created; migrations idempotent on rerun. — met
- Source gate rejects unapproved/unlinked/mismatched/unknown sources before any write. — met
- Parent-same-syllabus and no-cycle enforced before any write. — met
- Duplicate node code per syllabus rejected. — met
- Lifecycle transitions (draft→verified, draft/verified→retired) enforced; invalid transitions
  rejected. — met
- Role matrix: learner/unknown denied; editor read+draft only; reviewer adds verify/retire;
  admin all. — met
- Strict JSON (unknown fields / trailing JSON rejected on both POST and PATCH); no caller
  actor/reviewer field. — met
- Source gate re-validated on every node write, not only on `contentSourceId` change. — met
- List rejects unknown/inactive syllabus with `400 unknown_syllabus`; empty list only for a
  known active syllabus. — met
- Ancestor walk row-locks every traversed ancestor. — met
- No raw database/internal error text ever returned. — met
- No map data seeded; two seeded 0610/5090 sources remain pending/unlinked until a human
  completes rights approval + catalogue linking. — met
- Relevant tests pass. — met, see release validation below.

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No content, questions, OCR, source ingestion, or copyrighted data. No manual PATCH performed
  on the two seeded 0610/5090 content sources (human action only).
- No web UI for curriculum-map authoring in this task. No AI curriculum-map authority.

### Open questions / blockers

- None blocking. Authoring actual map content (topic labels, objective wording, etc.) requires
  a future private, approved editorial workflow — out of scope here, this task shipped
  infrastructure only. Linking/approving the two seeded 0610/5090 content sources remains a
  human action (carried from T-0005), not performed by this task.
- Observation, not fixed: a malformed (non-UUID) node id on `GET`/`PATCH`/`verify`/`retire`
  still yields `500 internal_error` rather than `404 not_found` — pre-existing behaviour shared
  with the catalogue and content-source packages, not one of the four review findings. Worth a
  separate cross-package task.

### Release validation (final pass, 2026-08-04)

| Command | Result |
| --- | --- |
| `go build ./... && go vet ./... && gofmt -l .` (Docker `golang:1.22-alpine`) | Pass — gofmt flags only pre-existing unrelated `internal/auth/auth_test.go` and `main_test.go` |
| `go test ./...` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource, curriculummap all green |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `docker compose -f docker-compose.test.yml up -d` + `pg_isready` wait | Pass — `sidus-test-postgres-test-1` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 11 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — catalogue (6) + content-source (4) + curriculum-map (17, incl. the concurrency ancestor-lock test) |
| `go run ./cmd/migrate` rerun against the same `sidus-test` | Pass — idempotent, 0 migrations applied |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; 6 routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Pass — clean |

### Handoff

`docs/handoffs/T-0006.md`

## T-0007 — Original question and rubric foundation

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0001 (done), T-0004 (done), T-0005 (done), T-0006 (done)

### Goal

Build private, metadata-and-code-only infrastructure for **original** questions and **versioned**
rubrics that future Exam Mode will use. Every question traces to exactly one **verified**
curriculum-map node whose approved content source still passes the T-0006 source gate.

**No question text, rubric text, syllabus text, mark schemes, past-paper content, PDFs,
extracted text, diagrams, OCR output, or derivative questions are created or seeded by this
task.** The public repository holds schema, code, contracts, docs, and tests only; question and
rubric content exists solely in a runtime database, written by a future **private editorial
workflow** that is out of scope here.

### Scope

- New tables `questions`, `question_rubric_versions`, `question_events` (migrations 0012–0014,
  plus additive 0015 for content revision), new package `services/core/internal/question`.
- Question fields: stable id, syllabus FK, curriculum-map-node FK, response type
  (`multiple_choice` | `short_answer` | `structured_response`), language, original question
  prompt/body, lifecycle status (`draft` | `verified` | `retired`), timestamps.
- Rubric version fields: stable id, question FK, immutable positive per-question version number,
  rubric structure JSONB (validation-safe schema), maximum marks, status (`draft` | `verified`),
  creator/reviewer verified Clerk subjects, timestamps.
- Question event fields: question id, event type covering create/update/verify/retire and
  rubric-version create/verify, verified Clerk subject, names-only changed fields, immutable
  trigger blocking `UPDATE`/`DELETE`. Never stores prompt or rubric values.
- Core API: list/get verified questions (by syllabus, optional node); create/PATCH draft
  question; create rubric version; list rubric versions (editorial roles only); verify rubric
  version; verify question; retire question.
- New least-privilege permissions `question:read`, `question:create`, `question:verify`,
  `question_rubric:read`. Learner/unknown denied.
- Shared TypeScript contracts, D-0009, `docs/question-rubric-model.md`, `docs/local-setup.md`,
  `CLAUDE.md`, handoff.

### Out of scope (explicitly not done)

- No AI generation, Anthropic calls, OCR, ingestion, or question derivation. The AI service is
  untouched.
- No human source-rights approval, catalogue linking, or curriculum-map authoring performed.
- No question, rubric, or map data seeded (no source currently passes the gate anyway).
- No web UI for question authoring.

### Schema decisions

- **Node link, not syllabus-only grounding.** `questions.curriculum_map_node_id` is a `NOT NULL`
  FK; `questions.syllabus_id` is also stored and must equal the node's syllabus, checked in the
  application layer on every write.
- **Verified-node + source gate re-run on every question write** (create, PATCH, verify, retire,
  rubric-version create, rubric-version verify).
- **`syllabusId` is immutable** on a question; re-point the node instead, which re-validates the
  syllabus match.
- **Rubric versions are append-only per question.** `UNIQUE (question_id, version)`, version
  allocated inside the write transaction under a row lock on the question, and a DB trigger
  rejects any `UPDATE` that changes `question_id`, `version`, `question_revision`, `rubric`, or
  `max_marks` — only `status`/`reviewed_by`/`updated_at` may change.
- **A rubric version is bound to the question content it was reviewed against** (review fix 1,
  migration 0015): `questions.content_revision` increments by exactly one per successful draft
  content update; every rubric version stores the revision current at its creation; a version
  counts towards question verification only while the two are equal.
- **Rubric JSONB has a validation-safe schema**, matched exactly and case-sensitively at every
  level (review fix 2): unknown keys, case variants, duplicate keys, wrongly-typed values, and
  trailing JSON are all rejected.
- **A question can only be verified when it has at least one verified rubric version for its
  current content revision** — no verified version at all is `409 missing_verified_rubric`;
  verified versions that all predate the current content are `409
  missing_current_verified_rubric`.
- **Retired questions disappear from reader endpoints** (verified-only reads).
- **`GET /questions` validates the optional node filter** (review fix 3): unknown/malformed →
  `400 unknown_node`, a real node of another syllabus → `400 mismatched_node`.

### Acceptance checks

- Migrations bootstrap on an empty database and are idempotent on rerun. — met
- Content revision increments exactly once per successful edit; never on `no_changes`, a
  rejected write, or a lifecycle transition. — met
- A question cannot be verified with only a rubric verified against an older revision; stale
  versions stay verified, immutable, and readable. — met
- Rubric JSON rejects case variants and duplicate keys at every level. — met
- The optional node filter on listing is validated. — met
- No seeded question or rubric text anywhere in the repository. — met
- Question syllabus must equal the mapped node's syllabus; draft/retired/missing nodes
  rejected. — met
- Verified node + source gate revalidated on every question write and verification. — met
- Question cannot be verified without a verified rubric version. — met
- Rubric versions immutable, unique, and monotonic per question. — met
- Lifecycle transitions enforced; invalid transitions rejected. — met
- Question events immutable; actor is the verified Clerk subject; names-only changed fields. —
  met
- Full role matrix: learner/unknown denied; editor read+draft+draft-rubric; reviewer adds
  verify/retire; admin all. — met
- Strict JSON: unknown fields, `actorId`/`reviewerId`, `syllabusId` change, lifecycle spoofing,
  and trailing JSON all rejected before any store call. — met
- No raw database/internal error text returned. — met
- Existing test suite stays green. — met

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.

### Open questions / blockers

- None blocking. Carried forward: the two seeded 0610/5090 content sources are still
  `pending`/unlinked (T-0001 approval + T-0005 linking), and no curriculum-map node has been
  authored or verified (T-0006). Until all three happen, no question can pass the grounding
  gate — which is why nothing is seeded.
- `language` is an opaque non-empty string (e.g. a BCP-47 tag); no language registry exists.
- Rubric listing is gated by its own permission (`question_rubric:read`) rather than reusing
  `question:read`.
- Cross-package observation, not fixed: `POST` handlers in `curriculummap`/`contentsource`/
  `catalogue` still accept case variants of known field names (`DisallowUnknownFields` is
  case-insensitive on struct decoding); the `question` package closes this with an explicit
  allowlist. A malformed (non-UUID) id still yields `500` in those sibling packages, unlike
  `question`. Both are candidates for a separate cross-package cleanup task.

### Release validation (final pass, 2026-08-05)

| Command | Result |
| --- | --- |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `go test ./...` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource, curriculummap, question all green |
| `docker compose -f docker-compose.test.yml up -d` + `pg_isready` wait | Pass — `sidus-test-postgres-test-1` healthy |
| `go run ./cmd/migrate` against disposable `sidus-test` | Pass — 15 migrations applied |
| `go run ./cmd/migrate` rerun against the same `sidus-test` | Pass — idempotent, 0 migrations applied |
| `go test ./... -run Integration -v` against `sidus-test` | Pass — catalogue + content-source + curriculum-map + question all green |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — Proxy (Middleware) detected; 6 routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Pass — clean |

### Handoff

`docs/handoffs/T-0007.md`

## T-0008 — Cross-package API input hardening

**Status:** done / released
**Owner:** Claude Code agent
**Priority:** P1
**Depends on:** T-0003 (done), T-0006 (done), T-0007 (done)

### Goal

Make the existing Core APIs in `contentsource`, `catalogue`, and `curriculummap` reject
malformed IDs and case-variant request fields consistently, closing the two cross-package gaps
the T-0007 review flagged as carried-forward observations, before any web editorial client is
built.

### Scope

- `services/core/internal/contentsource`, `services/core/internal/catalogue`,
  `services/core/internal/curriculummap` only.
- Every JSON body handler in these packages: reject unknown fields, case-variant field names,
  `actorId`/`reviewerId`, and trailing JSON — all before any store call, with the existing
  stable `invalid_json` error.
- Every route with an `{id}` path parameter: a malformed (non-UUID) id now maps to the same
  stable not-found response as a missing id, never a generic `500 internal_error`.
- `question` package: unchanged (already closed both gaps in T-0007).

### What changed

- `contentsource`, `catalogue`, `curriculummap` handlers: `decodeStrict` now takes an explicit
  case-sensitive field allowlist (decode into `map[string]json.RawMessage` first, reject any key
  outside the allowlist, then strict-decode into the destination struct) — mirrors the pattern
  `question` already used. Applied to every POST/PATCH body.
- `contentsource`, `catalogue` `postgres_store.go`: added `isInvalidTextRepresentation` (checks
  Postgres `22P02`) and used it alongside `sql.ErrNoRows` everywhere an `{id}` path parameter is
  looked up. `curriculummap` already had this helper (T-0006 review); its remaining `{id}`
  routes now use it too.

### Review fix (T-0008 review)

Every allowlist decoder decoded a literal JSON `null` body into a `nil`
`map[string]json.RawMessage` with no error, which passed the allowlist loop (zero keys) and
reached business validation looking identical to an empty object. Each decoder now rejects a
`nil` decoded map as `400 invalid_json` immediately after the first successful decode, before the
trailing-data check, the allowlist loop, or any store/event call. Valid `{}` is unaffected. See
D-0010 "Update" in `docs/decisions.md` and `docs/handoffs/T-0008.md` "Update (T-0008 review)".

### Acceptance checks

- Case-variant fields rejected for every affected create/PATCH body shape. — met
- `actorId`/`reviewerId`, unknown fields, trailing JSON values, null bodies, and trailing junk
  rejected before any store call, in every affected package. — met
- Malformed (non-UUID) IDs on GET/PATCH/verify/retire/approve/reject map to the existing
  not-found response, never `500`. — met
- A valid UUID for a missing resource still returns the existing not-found response
  (unchanged). — met
- Full role matrix and all pre-existing behavior remain green. — met

### Constraints

- Never stage/alter/move/delete: `DB.jpeg`, `arch.jpeg`, `Sidus.xlsx`,
  `Sidus_Roadmap_and_Cost_Model(1).xlsx`, `Sidus_Final_MVP_Technical_Cost_Model*.xlsx`,
  `Sidus_Final_MVP_Technical_Cost_Model_Recreated.xlsx`, `.claude/`, `.claude-flow/`, any
  `.env.local`.
- No business-rule, role, schema, migration, or source approval/linking changes. No UI, AI/OCR/
  ingestion, or source/PDF/text/diagram/question/rubric content work. No shared-helper
  extraction across packages.

### Open questions / blockers

None.

### Release validation (final pass, 2026-08-05)

| Command | Result |
| --- | --- |
| `docker compose -f docker-compose.yml config` / `-f docker-compose.test.yml config` | Pass / Pass |
| `go build ./... && go vet ./...` (Docker `golang:1.22-alpine`) | Pass |
| `gofmt -l .` (scoped to changed packages) | Pass — no changed file listed |
| `go test ./...` (unit, Docker toolchain) | Pass — core, auth, catalogue, contentsource, curriculummap, question all green |
| `docker compose -f docker-compose.test.yml up -d` + `pg_isready` wait | Pass — `sidus-test-postgres-test-1` healthy |
| `go run ./cmd/migrate` against a fresh disposable `sidus-test` | Pass — 15 migrations applied |
| `go run ./cmd/migrate` rerun | Pass — idempotent, 0 migrations applied |
| `go test ./... -run Integration` against `sidus-test` | Pass — catalogue, contentsource, curriculummap, question all green |
| `docker compose -f docker-compose.test.yml down -v` | Pass — `sidus-test` destroyed only; dev untouched |
| `python -m pytest -q` (services/ai) | Pass — 18 tests |
| `npm --prefix apps/web run typecheck` | Pass |
| `npm --prefix apps/web run build` | Pass — 6 routes intact |
| `npx -p typescript tsc --noEmit --strict packages/shared/src/contracts.ts` | Pass |
| `git diff --check` | Pass — clean |

### Handoff

`docs/handoffs/T-0008.md`
