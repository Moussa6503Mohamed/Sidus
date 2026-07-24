"""Clerk session authentication foundation for the AI service.

Clerk owns authentication; this module verifies a Clerk-issued session JWT presented as a
bearer token and exposes the verified identity to FastAPI routes. It performs signature,
expiry, issuer, and authorized-party checks against Clerk's JWKS (cached by PyJWKClient) —
never a Clerk Backend API call per request, and never any custom password handling.

The rights/provenance ingestion gate (`app.ingestion.guard_ingestion`) is unchanged and
remains the sole authority over whether a source may be ingested; this module only
authenticates the caller.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from functools import lru_cache
from typing import Any, Callable, Optional

import jwt
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer


@dataclass(frozen=True)
class Principal:
    """The verified caller identity: the Clerk subject and the `sidus_role` claim.

    The role is exposed for callers that need it, but the AI service's foundation only
    requires a valid session; role-based authorization of content-source actions is enforced
    by Sidus Core.
    """

    subject: str
    role: str


class ClerkAuthError(Exception):
    """Raised when a bearer token is missing, malformed, expired, or otherwise invalid."""


class ClerkConfigError(Exception):
    """Raised when the Clerk auth configuration is missing or unsafe.

    Signals a fail-closed condition (e.g. no issuer, or an explicitly blank authorized-parties
    list). The FastAPI layer turns this into a generic 503 without exposing details.
    """


# DEV_DEFAULT_AZP is the only `azp` origin accepted when CLERK_AUTHORIZED_PARTIES is absent:
# the local Sidus web origin. Production must set the env explicitly to non-local origin(s).
DEV_DEFAULT_AZP = "http://localhost:3000"


# KeyResolver resolves the verification key for a token. Production uses Clerk's JWKS; tests
# inject a resolver returning a local public key so verification runs offline.
KeyResolver = Callable[[str], Any]


class ClerkAuthenticator:
    """Verifies Clerk session JWTs.

    Args:
        issuer: expected token issuer (Clerk Frontend API URL). When set, tokens from any
            other issuer are rejected.
        authorized_parties: allow-list of accepted `azp` values (origins). When non-empty, a
            token whose `azp` is not listed is rejected.
        jwks_url: Clerk JWKS endpoint. Defaults to ``<issuer>/.well-known/jwks.json``.
        key_resolver: optional override that returns the verification key for a token; used
            by tests to verify offline without network access.
    """

    def __init__(
        self,
        issuer: Optional[str],
        authorized_parties: Optional[list[str]] = None,
        jwks_url: Optional[str] = None,
        key_resolver: Optional[KeyResolver] = None,
    ) -> None:
        self._issuer = (issuer or "").strip() or None
        if self._issuer is None:
            # Issuer is mandatory: a configured JWKS URL must never bypass issuer validation.
            raise ValueError("issuer is required for Clerk verification")
        self._authorized_parties = [p for p in (authorized_parties or []) if p.strip()]
        if key_resolver is not None:
            self._key_resolver: KeyResolver = key_resolver
        else:
            resolved_jwks_url = jwks_url
            if resolved_jwks_url is None and self._issuer:
                resolved_jwks_url = self._issuer.rstrip("/") + "/.well-known/jwks.json"
            if resolved_jwks_url is None:
                raise ValueError("issuer or jwks_url or key_resolver is required")
            jwk_client = jwt.PyJWKClient(resolved_jwks_url)
            self._key_resolver = lambda token: jwk_client.get_signing_key_from_jwt(token).key

    def verify(self, token: str) -> Principal:
        """Verify token and return the Principal, or raise ClerkAuthError."""
        if not token or not token.strip():
            raise ClerkAuthError("empty token")

        try:
            key = self._key_resolver(token)
        except Exception as exc:  # noqa: BLE001 - any resolver failure is an auth failure
            raise ClerkAuthError(f"could not resolve signing key: {exc}") from exc

        # Issuer is always set (enforced in __init__), so issuer validation is always on — a
        # configured JWKS URL cannot bypass it.
        options = {"require": ["exp", "sub", "iss"], "verify_iss": True}
        try:
            payload = jwt.decode(
                token,
                key,
                algorithms=["RS256"],
                issuer=self._issuer,
                options=options,
            )
        except jwt.PyJWTError as exc:
            raise ClerkAuthError(f"token verification failed: {exc}") from exc

        azp = payload.get("azp")
        if self._authorized_parties and azp not in self._authorized_parties:
            raise ClerkAuthError("authorized party not allowed")

        subject = payload.get("sub")
        if not subject:
            raise ClerkAuthError("token missing subject")

        role = payload.get("sidus_role") or ""
        return Principal(subject=subject, role=str(role))


def _split_env_list(raw: str) -> list[str]:
    return [part.strip() for part in raw.split(",") if part.strip()]


def _authorized_parties_from_env() -> list[str]:
    """Resolve accepted `azp` origins from CLERK_AUTHORIZED_PARTIES.

    Absent → the local dev default only. Present but resolving to zero valid origins after
    trimming → ``ClerkConfigError`` (fail closed): an explicitly blank value must never yield
    an unrestricted azp check.
    """
    raw = os.getenv("CLERK_AUTHORIZED_PARTIES")
    if raw is None:
        return [DEV_DEFAULT_AZP]
    parties = _split_env_list(raw)
    if not parties:
        raise ClerkConfigError("CLERK_AUTHORIZED_PARTIES is set but empty")
    return parties


@lru_cache
def get_authenticator() -> Optional[ClerkAuthenticator]:
    """Build the process-wide authenticator from the environment.

    Returns ``None`` when the configuration is missing or unsafe (issuer absent/blank, or an
    explicitly blank authorized-parties list) so protected routes fail closed with a generic
    503. Overridable in tests via ``app.dependency_overrides[get_authenticator]``.
    """
    issuer = os.getenv("CLERK_JWT_ISSUER")
    if not issuer or not issuer.strip():
        return None
    try:
        parties = _authorized_parties_from_env()
        return ClerkAuthenticator(
            issuer=issuer,
            authorized_parties=parties,
            jwks_url=os.getenv("CLERK_JWKS_URL") or None,
        )
    except (ClerkConfigError, ValueError):
        return None


_bearer_scheme = HTTPBearer(auto_error=False)


def require_clerk_session(
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(_bearer_scheme),
    authenticator: Optional[ClerkAuthenticator] = Depends(get_authenticator),
) -> Principal:
    """FastAPI dependency that requires a valid Clerk session bearer token.

    Returns the verified Principal. Fails closed with a generic 503 when auth is not safely
    configured, and 401 for a missing/invalid token. Neither response exposes configuration
    details or secrets.
    """
    if authenticator is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="authentication is not configured",
        )
    if credentials is None or not credentials.credentials:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="a Clerk session bearer token is required",
            headers={"WWW-Authenticate": "Bearer"},
        )
    try:
        return authenticator.verify(credentials.credentials)
    except ClerkAuthError as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="session token is missing, invalid, or expired",
            headers={"WWW-Authenticate": "Bearer"},
        ) from exc
