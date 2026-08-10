# Local authenticated browser E2E harness (T-0025)

A committed, reproducible browser end-to-end harness that drives the **real signed-in UI → BFF →
Core** path against the isolated local HTTPS stack from T-0023. It exists so the full editorial
journey — register a source, approve it, author and verify a curriculum node, author an MCQ,
version and verify its rubric, choose a canonical rubric, verify the question, then answer it as a
learner and read the feedback — can be re-run on demand instead of hand-clicked.

Everything here is **local only**. It is not wired into CI, it talks to nothing outside loopback,
and it starts and stops nothing: the operator owns the Docker stack and the dev server, and both
are left running afterwards.

## What this harness will never do

These are constraints on the design, not just current behaviour:

- **No auth bypass.** There is no test-only sign-in route, no fake session, no injected role, no
  `NEXT_PUBLIC_*` escape hatch. Every authenticated run uses cookies from a real Clerk sign-in
  performed by a human in a real browser.
- **No credentials in this repository.** No Clerk password, secret key, bearer token, cookie, or
  storage-state file is read from, written to, or stageable in the repo. Captured state lives only
  under `D:\Sidus-private-content\e2e`, and the harness refuses to read or write it anywhere
  inside the repo (`assertPrivateRoot`).
- **No credential leakage into artifacts.** Playwright traces, videos, screenshots, and
  error-context snapshots of a signed-in page embed live session cookies, so every artifact stream
  is off and `outputDir` points outside the repository.
- **No weakened TLS.** `ignoreHTTPSErrors` stays `false`. Core is reached over real TLS and the
  private dev CA is trusted through `NODE_EXTRA_CA_CERTS`, exactly as the T-0022 Python client
  does through `SIDUS_CORE_CA_BUNDLE`.
- **No educational or source-derived content.** Every record it creates is built from opaque
  runtime nonces (`SIDUS-E2E-SYNTHETIC-<nonce>-<field>`) and asserted to be so before the first
  keystroke. Nothing is seeded in a migration and nothing is committed.
- **No teardown.** It never runs `docker compose down`, never deletes a record, and never touches
  the dev (`sidus`) or Go test (`sidus-test`) stacks.

## Prerequisites

### 1. The isolated local HTTPS Core stack

Set up and started per [`local-import-test-environment.md`](local-import-test-environment.md),
including the private dev TLS CA under `D:\Sidus-private-content\local-dev`.

```sh
docker compose -f docker-compose.local-import.yml --env-file .env.local-import up -d
```

**One extra setting is needed for this harness.** The web app runs on port **3001** here, so the
token's `azp` claim is `http://localhost:3001`, but the stack defaults to accepting
`http://localhost:3000` only. Add this to `.env.local-import` (gitignored) and restart the stack,
or every authenticated Core call fails with `401`:

```
CLERK_AUTHORIZED_PARTIES=http://localhost:3000,http://localhost:3001
```

### 2. The web app on port 3001

Started by you, in its own terminal, pointed at the HTTPS Core stack and trusting the private CA:

```powershell
$env:SIDUS_CORE_API_URL = "https://127.0.0.1"
$env:NODE_EXTRA_CA_CERTS = "D:\Sidus-private-content\local-dev\ca.pem"
npm --prefix apps/web run dev -- --port 3001
```

`NODE_EXTRA_CA_CERTS` is required on **both** this process and the harness process: the BFF's
server-side `fetch` to Core and the harness's own preflight each verify the chain independently.

### 3. Clerk users, one per profile

The harness has three profiles. Create a Clerk user for each in the same dev instance the stack is
configured against, and set `sidus_role` public metadata per [`auth-setup.md`](auth-setup.md):

| Profile | `sidus_role` | Used for |
| --- | --- | --- |
| `admin` | `admin` | The full editorial → practice journey (needs both editing and reviewing permissions) |
| `learner` | `learner` | Editorial denial, Practice still allowed |
| `unknown` | *(unset / not a recognized role)* | Deny-by-default: refused everywhere, including Practice |

## Capture a session (once per profile, roughly every few hours)

```powershell
$env:SIDUS_E2E_PROFILE = "admin"     # then repeat for "learner" and "unknown"
npm --prefix apps/web run e2e:auth
```

A headed browser opens on `/sign-in` and waits up to ten minutes. **You** sign in — the harness
never sees, stores, or types a password. Once it lands on `/dashboard` and confirms the session
survives a fresh server-rendered request, it writes:

```
D:\Sidus-private-content\e2e\storage-state.<profile>.json        # the cookies Playwright captured
D:\Sidus-private-content\e2e\storage-state.<profile>.meta.json   # capture time only
```

Nothing about the captured state is printed — only the profile name.

## Run

```powershell
$env:NODE_EXTRA_CA_CERTS = "D:\Sidus-private-content\local-dev\ca.pem"
npm --prefix apps/web run e2e
```

| Project | Profile | Covers |
| --- | --- | --- |
| `fail-closed` | *(none)* | Signed-out redirects, both BFFs refusing anonymous callers, a forged/expired session cookie buying no trust |
| `journey-admin` | `admin` | The full ten-step editorial → learner journey |
| `denial-learner` | `learner` | Editorial pages **and the editorial BFF** refused; Practice still reachable |
| `denial-unknown` | `unknown` | Editorial *and* Practice refused, both in the UI and at Core |

The denial projects assert at the BFF/Core layer as well as the UI on purpose: the web-side role
check is cosmetic by design (D-0006/D-0011), so "the page says No access" proves nothing on its
own. A `403` from Core is the assertion that matters.

### Narrowing a run

`SIDUS_E2E_PROFILES` selects which authenticated projects run. Every profile is required by
default, so a missing capture aborts the run rather than silently shrinking it.

```powershell
$env:SIDUS_E2E_PROFILES = "admin"    # only the journey (plus fail-closed, which needs no session)
$env:SIDUS_E2E_PROFILES = "none"     # only fail-closed — needs no captured session at all
```

### Other environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `SIDUS_E2E_STATE_DIR` | `D:\Sidus-private-content\e2e` | Where captured state lives. Must be outside the repo. |
| `SIDUS_E2E_WEB_URL` | `http://localhost:3001` | The running web app. |
| `SIDUS_E2E_CORE_URL` | `https://127.0.0.1` | The local HTTPS Core stack. |
| `SIDUS_E2E_SYLLABUS_CODE` | `9700` | Which active catalogue syllabus the journey authors against. |

## What the journey actually does

One test, ten steps, all through the real UI as the `admin` profile:

1. Register a **pending** synthetic content source with complete rights metadata, linked to the
   catalogue syllabus.
2. Approve it (reviewer action; Core re-checks every required rights field).
3. Create a draft curriculum-map node grounded in that approved source.
4. Verify the node.
5. Create a multiple-choice question draft grounded in the verified node, origin `original`.
6. Create a draft rubric version with a correct option, a correct-answer explanation, and an
   explanation for every incorrect option.
7. Verify the rubric version (its content becomes immutable).
8. Select the canonical rubric version and verify the question — one atomic reviewer action
   (T-0014).
9. Open Practice, pick the syllabus, load questions, and find the now-eligible question.
10. Select an option, submit, and read the score plus every canonical explanation back.

Each run creates one source, one node, one question, one rubric version, and one attempt in the
**local** `sidus-local-import` database, all tagged with that run's nonce. They are left in place —
the harness never deletes anything. Wipe them with the stack's own scoped teardown when you want a
clean slate:

```sh
docker compose -f docker-compose.local-import.yml --env-file .env.local-import down -v
```

## Tool tests

The harness's own libraries are covered by the normal Vitest suite (`npm --prefix apps/web run
test`), not by the browser run:

- `e2e/lib/storage-state.test.ts` — fail-closed behaviour: missing file, unparseable file, no
  Clerk cookie, expired cookies, a capture older than the age limit, missing/damaged/mismatched
  capture-time sidecar, a state directory inside the repository, and the guarantee that no cookie
  value reaches an error message.
- `e2e/lib/synthetic.test.ts` — fixtures are opaque, unique per run, within Core's option-id limit,
  and `assertSynthetic` rejects anything resembling real prose.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `Core at https://127.0.0.1 is running but its TLS certificate does not verify` | `NODE_EXTRA_CA_CERTS` is not set on the harness process. Set it, then re-run. |
| `Core is not reachable at https://127.0.0.1` | The `sidus-local-import` stack is down — bring it up. |
| `the Sidus web app is not reachable at http://localhost:3001` | The dev server is not running, or is on another port. |
| `no captured browser state for profile "…"` | Capture it (see above), or narrow `SIDUS_E2E_PROFILES`. |
| `…was captured Nh ago (limit 8h) and is treated as stale` | Re-capture. Age is checked independently of cookie expiry because a Clerk session can be revoked server-side. |
| Authenticated steps fail with `401` from Core | `CLERK_AUTHORIZED_PARTIES` in `.env.local-import` does not include `http://localhost:3001`. |
| `Assertion failed: !(handle->flags & UV_HANDLE_CLOSING)` after a preflight failure | Cosmetic Windows/Node noise from Playwright's own shutdown. The actionable message is printed just above it, and the run does exit non-zero. |
