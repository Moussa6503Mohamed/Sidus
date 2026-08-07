# Local-only HTTPS Core test environment (T-0023)

Purpose: let the private T-0022 pending-source import tool
(`D:\Sidus-private-content\tools`) be exercised against a **real running Core over real
HTTPS/TLS with real Clerk-auth-gated routes**, entirely on loopback, before anyone runs the
full 489-record `--apply`. This environment creates **no** `content_sources`, approval,
question, rubric, node, or attempt row — it is purely infrastructure to prove the path works.

Isolated from both the dev stack (`docker-compose.yml`, project `sidus`) and the disposable Go
test stack (`docker-compose.test.yml`, project `sidus-test`): its own Compose project
(`sidus-local-import`), its own Postgres, its own network/volumes. Only the HTTPS reverse proxy
publishes a port, and only on `127.0.0.1`.

**The reverse proxy binds `127.0.0.1:443`, not a non-standard port.** This is deliberate: the
T-0022 client's `_canonicalize_base_url` has an existing, explicitly tested contract that
rejects any HTTPS origin with a non-default port. Using `443` (loopback-only) lets
`SIDUS_CORE_API_URL=https://127.0.0.1` work with that validator completely unchanged — see
`docs/decisions.md` D-0022. If something else on your machine already binds `127.0.0.1:443`,
stop that service for the duration of this test rather than changing the port here.

## Required ignored environment variables

Copy the tracked placeholder file, then fill in real values — **never commit the copy**:

```
cp .env.local-import.example .env.local-import
```

`.env.local-import` (gitignored, same pattern as `.env.local`/`.env`):

| Variable | Purpose |
| --- | --- |
| `CLERK_SECRET_KEY` | Same value as `apps/web/.env.local` — same Clerk dev application, so a token minted by signing in through the web app verifies here. |
| `CLERK_JWT_ISSUER` | Same value as `apps/web/.env.local`'s Clerk instance issuer. |
| `CLERK_AUTHORIZED_PARTIES` | Optional; defaults to `http://localhost:3000` (matches the web app's origin, which is where the token's `azp` claim comes from). |
| `SIDUS_LOCAL_TLS_DIR` | Optional; defaults to `D:/Sidus-private-content/local-dev`. Directory containing the private dev TLS cert/key (below). Never point this at a path inside this repo. |

## Generate the private dev TLS certificate (one-time)

Never committed, never printed, never copied into this repo — lives only under
`D:\Sidus-private-content\local-dev`. Uses `openssl` only (already present in Git Bash on
Windows) — no new tool to install, no system trust-store changes, no admin rights needed.

```sh
export MSYS_NO_PATHCONV=1   # Git Bash only: prevents /CN=... from being read as a path
OUT_DIR="D:\Sidus-private-content\local-dev"
mkdir -p "$OUT_DIR" && cd "$OUT_DIR"

# Private root CA (this file's cert half — ca.pem — is what SIDUS_CORE_CA_BUNDLE points at)
openssl genrsa -out ca-key.pem 4096
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 825 \
  -out ca.pem -subj "/CN=Sidus Local Import Dev CA"

# Leaf cert for 127.0.0.1 / localhost, signed by that CA
openssl genrsa -out server-key.pem 2048
cat > server-san.cnf <<'EOF'
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no
[req_distinguished_name]
CN = 127.0.0.1
[v3_req]
keyUsage = keyEncipherment, digitalSignature
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
EOF
openssl req -new -key server-key.pem -out server.csr -config server-san.cnf
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial \
  -out server-cert.pem -days 825 -sha256 -extfile server-san.cnf -extensions v3_req
rm -f server.csr server-san.cnf ca.srl
```

Result: `ca.pem`, `ca-key.pem`, `server-cert.pem`, `server-key.pem` in
`D:\Sidus-private-content\local-dev`. The proxy uses `server-cert.pem`/`server-key.pem`; the
T-0022 client's `SIDUS_CORE_CA_BUNDLE` points at `ca.pem`.

## Start / stop

```sh
# Start (applies migrations to the isolated DB, then starts Core and the HTTPS proxy)
docker compose -f docker-compose.local-import.yml --env-file .env.local-import up -d

# Stop — scoped teardown, removes ONLY sidus-local-import project resources
docker compose -f docker-compose.local-import.yml --env-file .env.local-import down -v
```

## Health check

```sh
curl --cacert D:\Sidus-private-content\local-dev\ca.pem https://127.0.0.1/healthz
# -> {"service":"core","status":"ok"}
```

Without `--cacert` (or any CA that doesn't chain to your private CA), the TLS handshake fails —
this cert is not in your system trust store, by design.

**Windows-native `curl.exe` (Schannel backend) note:** Schannel treats "revocation status
unknown" as a hard failure by default, and this private CA has no CRL/OCSP endpoint (normal for
a throwaway dev CA). If you see `curl: (60) schannel: the revocation status is unknown`, add
`--ssl-no-revoke` to the command above. This is a Windows-curl-only quirk around revocation
checking — it does not affect certificate/hostname validation, and it does not affect the
Python client at all: `ssl.create_default_context` (used by `SIDUS_CORE_CA_BUNDLE`) never
performs revocation checking in the first place.

## Sign in locally with Clerk

Run the web app against the **same** Clerk dev instance as this stack's `.env.local-import`:

```sh
cd apps/web && npm install && npm run dev
```

Open `http://localhost:3000`, sign in with a Clerk user whose `sidus_role` public metadata is
`editor` or `admin` (see `docs/auth-setup.md`), then visit `/dashboard` to confirm you're
signed in.

## Obtain a short-lived token without committing or logging it

While signed in at `http://localhost:3000/dashboard`, open the browser DevTools console and run:

```js
await window.Clerk.session.getToken()
```

Copy the returned JWT directly into an interactive shell variable — never into a file, never
echoed by a script, never pasted into a commit or a chat log:

```powershell
$env:SIDUS_CLERK_BEARER_TOKEN = "<paste the token here>"
```

The token is short-lived; repeat this before each manual test session.

## Dry run (safe, zero HTTP calls, unaffected by any of the above)

```sh
cd D:\Sidus-private-content\tools
python register_pending_sources.py
```

## One-record smoke-test procedure (documented only — do not execute yet)

`register_pending_sources.py --apply` cannot be used for a one-record smoke test: it refuses to
apply unless the manifest validates to **exactly 489** valid pairs, with no override flag, by
design (see the tool's `README.md`). Instead, once the stack above is healthy and you have a
real bearer token, use `ContentSourceClient` directly with one synthetic record:

```python
import os
from source_registration.api_client import ContentSourceClient

client = ContentSourceClient(
    base_url="https://127.0.0.1",
    bearer_token=os.environ["SIDUS_CLERK_BEARER_TOKEN"],
    ca_bundle_path=r"D:\Sidus-private-content\local-dev\ca.pem",
)
result = client.create_pending_source({
    "title": "smoke-test — safe to reject",
    "sourceUrl": "https://example.invalid/smoke-test",
    "sourceHash": "0" * 64,
    "syllabusCode": "9700",
})
print(result)
```

This creates exactly one pending `content_sources` row on the local stack. Reject it afterward
through the editorial review workflow (`/dashboard/editorial/sources`, or the underlying reject
endpoint) — the tool's own `README.md` "Rollback" section explains why nothing is ever deleted
in place.

**This procedure is documented only. It was not executed as part of T-0023, and the full
489-record `--apply` against real Core remains a separate, explicit, later human action.**
