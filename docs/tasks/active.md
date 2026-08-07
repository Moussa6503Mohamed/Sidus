# Active tasks

## T-0023 — Local-only HTTPS Core test environment for pending-source import

**Status:** review
**Owner:** Claude Code agent, 2026-08-07

Isolated `docker-compose.local-import.yml` (project `sidus-local-import`) stack — own Postgres,
migration run, Core, and a Caddy HTTPS reverse proxy bound `127.0.0.1:443` only — for exercising
the private T-0022 pending-source import tool against a real running Core over real TLS with
real Clerk auth, before anyone runs the real 489-record `--apply`. Private dev TLS CA/cert live
only under `D:\Sidus-private-content\local-dev`. T-0022's `api_client.py` gains one optional,
additive `SIDUS_CORE_CA_BUNDLE`-backed `ca_bundle_path` parameter; unset behavior is unchanged.
See `docs/decisions.md` D-0022, `docs/local-import-test-environment.md`, and
`docs/handoffs/T-0023.md`.

No `content_sources`, approval, question, rubric, node, or attempt row was created. The
documented one-record smoke-test procedure and the full 489 `--apply` were **not** executed —
both remain separate, explicit, later human actions.

### Open questions / blockers

None.
