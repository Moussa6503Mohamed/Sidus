# Authentication and roles (T-0003)

Clerk owns authentication. Sidus Core owns authorization. No endpoint trusts a
caller-supplied `actorId`/`reviewerId`; the audit actor and review reviewer are always the
verified Clerk session **subject**.

## Secrets rule

- Real Clerk keys live only in **gitignored** `.env.local` files. Never commit, stage, log,
  or paste a key into docs.
- `.env.example` holds placeholders only.
- `apps/web/.env.local` is for the local Next.js runtime and is gitignored — never stage it.

## Clerk Dashboard manual setup (do once, before beta)

1. **Create the application** in the Clerk Dashboard and copy the **Publishable key**
   (`pk_...`) and **Secret key** (`sk_...`) into the gitignored `.env.local` files (see
   "Local setup" below). Never put real keys in `.env.example`.
2. **Add the `sidus_role` session claim.**
   - Dashboard → **Sessions** → **Customize session token**.
   - Add a claim named exactly **`sidus_role`** with value `{{user.public_metadata.sidus_role}}`
     (store each user's role in their **public metadata** under `sidus_role`).
   - The claim value must be one of: `learner`, `editor`, `reviewer`, `admin`.
3. **Roles and least privilege** (enforced by Sidus Core, not Clerk):

   | Role | Content-source permissions |
   | --- | --- |
   | `learner` | none |
   | `editor` | read, create, PATCH pending sources |
   | `reviewer` | editor permissions **plus** approve/reject |
   | `admin` | all content-source permissions |
   | missing / unknown | **denied by default** (no access) |

4. **Add the first admin manually before beta.** No self-service role escalation exists. In
   the Dashboard, open the intended admin user and set their public metadata
   `{"sidus_role": "admin"}`. Every other user defaults to no content-source access until an
   admin/reviewer flow grants a role.
5. **Production origins / domains.** In the Clerk Dashboard configure your production domain
   and allowed origins. In Sidus Core set `CLERK_AUTHORIZED_PARTIES` to the exact production
   web origin(s) (comma-separated) and `CLERK_JWT_ISSUER` to your production Frontend API
   URL. Development defaults to `http://localhost:3000` only.

## Local setup

1. Copy env template and fill real keys into **gitignored** files:
   - Root: `cp .env.example .env` then set `CLERK_JWT_ISSUER`, `CLERK_AUTHORIZED_PARTIES`,
     `CLERK_JWKS_URL`, and `CLERK_SECRET_KEY` for the Core/AI services.
   - Web: create `apps/web/.env.local` with `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` and
     `CLERK_SECRET_KEY` (plus the sign-in/up URL vars). This file is gitignored.
2. Run the web app: `cd apps/web && npm install && npm run dev` → http://localhost:3000.
   - Signed-out home shows **Sign in** / **Sign up**. Signed-in home shows the user menu and
     a **Dashboard** link. `/dashboard` is protected by the Clerk proxy (`apps/web/proxy.ts`).
3. Run Core with Clerk configured (content-source routes only mount when **both**
   `DATABASE_URL` and `CLERK_SECRET_KEY` are set — fail closed otherwise).
4. Run AI (`services/ai`); `/ingestion/status` requires a valid session, `/healthz` is public.

## Token flow

1. The user signs in via Clerk in the web app. Clerk issues a short-lived session JWT signed
   with the instance's private key and containing `sub`, `iss`, `azp`, `exp`, and the
   `sidus_role` claim.
2. A client calls a Sidus API with `Authorization: Bearer <session-token>`.
3. The backend verifies the token **offline** against Clerk's JWKS (public keys), cached by
   key id — no Clerk Backend API call per request.
4. The backend derives identity and role from the **verified** claims only.

## Backend verification

Both backends validate **signature, expiry, issuer, and authorized party (`azp`)**.

- **Core (Go, `services/core/internal/auth`)** uses the official Clerk Go SDK
  (`clerk-sdk-go/v2`): decode → resolve+cache the signing JWK by `kid` → `jwt.Verify`
  (signature/expiry/issuer/azp) → pin issuer to `CLERK_JWT_ISSUER` → read `sub` and
  `sidus_role`. `auth.Protect` wraps every content-source route with a required permission:
  - missing/invalid token → **401**
  - valid token, role lacks the permission → **403**
  - on success the verified subject is attached to the request context and used as the audit
    actor/reviewer.
- **AI (Python, `services/ai/app/auth.py`)** verifies the bearer token against Clerk's JWKS
  (PyJWT `PyJWKClient`, RS256), checks issuer/azp/expiry, and exposes the verified
  `Principal(subject, role)` via the `require_clerk_session` FastAPI dependency. The
  rights/provenance ingestion gate is unchanged; the protected `/ingestion/status` route is
  the authenticated foundation for future ingestion endpoints (no OCR/content ingestion).

## Role → permission matrix (authoritative)

| Permission | learner | editor | reviewer | admin |
| --- | --- | --- | --- | --- |
| read sources | ✗ | ✓ | ✓ | ✓ |
| create source | ✗ | ✓ | ✓ | ✓ |
| PATCH pending source | ✗ | ✓ | ✓ | ✓ |
| approve / reject | ✗ | ✗ | ✓ | ✓ |

Unknown/missing role: denied everywhere.

## Environment variables

| Variable | Service | Purpose |
| --- | --- | --- |
| `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY` | web | Clerk publishable key (public). |
| `CLERK_SECRET_KEY` | web, core | Clerk secret (JWKS fetch / SSR). **Never commit.** |
| `NEXT_PUBLIC_CLERK_SIGN_IN_URL` / `..._SIGN_UP_URL` | web | Mounted Clerk routes. |
| `CLERK_JWT_ISSUER` | core, ai | Pinned token issuer (Frontend API URL). |
| `CLERK_AUTHORIZED_PARTIES` | core | Comma-separated accepted `azp` origins. Dev: local only. |
| `CLERK_JWKS_URL` | ai | JWKS endpoint; defaults to `<issuer>/.well-known/jwks.json`. |
