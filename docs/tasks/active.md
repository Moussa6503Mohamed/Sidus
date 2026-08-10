# Active tasks

## T-0024 — Local TLS certificate generation defect fix (D-0022 update)

**Status:** review
**Owner:** Claude Code agent, 2026-08-10
**Depends on:** T-0023 (done/released — this fixes a defect in its cert-generation doc)

**Scope:** fix `docs/local-import-test-environment.md`'s private dev CA/leaf certificate
generation so the CA carries a `keyUsage` extension (root cause of
`ssl.create_default_context` raising `CERTIFICATE_VERIFY_FAILED: CA cert does not include key
usage extension`), regenerate only the private cert files under
`D:\Sidus-private-content\local-dev`, and update T-0023's handoff/decisions.md to record the
fix. No Compose/Caddy/Core/AI/client code change; no schema/migration/business-rule change.

**Acceptance checks:** see `docs/handoffs/T-0023.md` "Update (T-0024)" validation table.

**Handoff:** `docs/handoffs/T-0023.md` "Update (T-0024)". See `docs/decisions.md` D-0022
"Update (T-0024)".
