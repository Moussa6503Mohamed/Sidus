# MVP execution plan — 11 August 2026

## Current shipped foundation

- Clerk authentication and role authorization.
- Rights/provenance gates, curriculum modules, editorial authoring, verified rubrics.
- Learner Practice and Exam with Module selection, flexible counts, deterministic answers, and
  pending-review written responses.
- Local authenticated E2E, private source metadata intake, and local 9700 setup.

## Ordered delivery tracks

| Order | Task | Outcome |
| --- | --- | --- |
| 1 | T-0030 private upload and review intake | Complete. Admin uploads private PDFs to quarantined object storage; scan, MIME/size checks, retention, immutable metadata, review queue. No public file serving. |
| 2 | T-0031 brand-kit alignment | Audit and correct every learner/editorial screen against Sidus Observatory tokens, logo, typography, layout, states, and responsive rules. |
| 3 | T-0032 persistent assessments | Server assessment session, autosave, resume/reconnect, immutable item order, idempotent finalization. |
| 4 | T-0033 Sonnet review adapter and evaluation gate | Anthropic Sonnet adapter, strict structured output, version/cost/confidence trace, automated quality gate; disabled until API key configured and evaluation passes. |
| 5 | T-0034 Sonnet written-response marking | Sonnet produces final criterion-level marks, feedback, confidence, and immutable trace. Invalid or low-confidence work is withheld or retried automatically; no human marking workflow. |
| 6 | T-0035 learning analytics | Durable learning events, subject/module aggregates, learner dashboard. |
| 7 | T-0036 teacher and consent | Classes, invitations, explicit acceptance/revocation, assignments, scoped views. |
| 8 | T-0037 learner experience completion | Tutor/Test rules, Arabic/RTL, offline-safe shell, accessibility/mobile audit. |
| 9 | T-0038 beta operations | Feature flags, telemetry, backups/restore, rate limits, security/load checks, support runbooks. |

## Deferred until dedicated designs

- Matching, tables, graphs, diagrams, handwriting, file-answer attachments, question finder,
  notifications, reports, parent/organization views, billing, public catalog scale-out.

## Non-negotiable gates

- Uploaded source PDFs stay private; never Git, Notion, browser learner routes, or logs.
- Each source must pass rights/provenance review before any downstream processing.
- Sonnet output passes schema, rubric, and confidence gates before release. Invalid or
  low-confidence results are withheld or retried automatically; no fabricated score.
- Sidus is AI-first: ingestion, classification, mapping, question drafting, marking,
  explanations, feedback, recommendations, and quality gates are automated. Human marking exists
  only when a teacher explicitly configures a test for manual checking.
- Deterministic marking stays primary; uncertain/open work routes to human review.
- Every task requires tests, independent review, handoff, and release validation.
