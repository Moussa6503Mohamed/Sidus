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

        options = {"require": ["exp", "sub"], "verify_iss": self._issuer is not None}
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


@lru_cache
def get_authenticator() -> ClerkAuthenticator:
    """Build the process-wide authenticator from the environment.

    Overridable in tests via ``app.dependency_overrides[get_authenticator]``.
    """
    return ClerkAuthenticator(
        issuer=os.getenv("CLERK_JWT_ISSUER"),
        authorized_parties=_split_env_list(os.getenv("CLERK_AUTHORIZED_PARTIES", "")),
        jwks_url=os.getenv("CLERK_JWKS_URL") or None,
    )


_bearer_scheme = HTTPBearer(auto_error=False)


def require_clerk_session(
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(_bearer_scheme),
    authenticator: ClerkAuthenticator = Depends(get_authenticator),
) -> Principal:
    """FastAPI dependency that requires a valid Clerk session bearer token.

    Returns the verified Principal, or raises 401 for a missing/invalid token.
    """
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
