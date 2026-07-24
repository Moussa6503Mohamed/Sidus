"""Offline tests for the Clerk session auth foundation.

A local RSA key pair signs test tokens and a key_resolver injects the matching public key,
so verification runs without any network access or live Clerk instance.
"""

from __future__ import annotations

import datetime as dt

import jwt
import pytest
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi.testclient import TestClient

from app.auth import (
    DEV_DEFAULT_AZP,
    ClerkAuthenticator,
    ClerkAuthError,
    ClerkConfigError,
    _authorized_parties_from_env,
    get_authenticator,
    require_clerk_session,
)
from app.main import app

ISSUER = "https://example.clerk.accounts.dev"
AZP = "http://localhost:3000"


@pytest.fixture(scope="module")
def rsa_key() -> rsa.RSAPrivateKey:
    return rsa.generate_private_key(public_exponent=65537, key_size=2048)


def _make_token(
    private_key: rsa.RSAPrivateKey,
    *,
    sub: str = "user_123",
    role: str | None = "editor",
    issuer: str = ISSUER,
    azp: str | None = AZP,
    expires_in: int = 300,
) -> str:
    now = dt.datetime.now(tz=dt.timezone.utc)
    payload: dict[str, object] = {
        "sub": sub,
        "iss": issuer,
        "iat": now,
        "exp": now + dt.timedelta(seconds=expires_in),
    }
    if azp is not None:
        payload["azp"] = azp
    if role is not None:
        payload["sidus_role"] = role
    return jwt.encode(payload, private_key, algorithm="RS256", headers={"kid": "test-key"})


def _authenticator(rsa_key: rsa.RSAPrivateKey) -> ClerkAuthenticator:
    public_key = rsa_key.public_key()
    return ClerkAuthenticator(
        issuer=ISSUER,
        authorized_parties=[AZP],
        key_resolver=lambda _token: public_key,
    )


def test_verify_returns_principal(rsa_key: rsa.RSAPrivateKey) -> None:
    token = _make_token(rsa_key, sub="user_abc", role="reviewer")
    principal = _authenticator(rsa_key).verify(token)
    assert principal.subject == "user_abc"
    assert principal.role == "reviewer"


def test_verify_missing_role_defaults_empty(rsa_key: rsa.RSAPrivateKey) -> None:
    token = _make_token(rsa_key, role=None)
    principal = _authenticator(rsa_key).verify(token)
    assert principal.role == ""


def test_verify_rejects_expired(rsa_key: rsa.RSAPrivateKey) -> None:
    token = _make_token(rsa_key, expires_in=-10)
    with pytest.raises(ClerkAuthError):
        _authenticator(rsa_key).verify(token)


def test_verify_rejects_wrong_issuer(rsa_key: rsa.RSAPrivateKey) -> None:
    token = _make_token(rsa_key, issuer="https://attacker.example.com")
    with pytest.raises(ClerkAuthError):
        _authenticator(rsa_key).verify(token)


def test_verify_rejects_unauthorized_party(rsa_key: rsa.RSAPrivateKey) -> None:
    token = _make_token(rsa_key, azp="https://evil.example.com")
    with pytest.raises(ClerkAuthError):
        _authenticator(rsa_key).verify(token)


def test_verify_rejects_bad_signature(rsa_key: rsa.RSAPrivateKey) -> None:
    other_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    token = _make_token(other_key)  # signed by a different key than the resolver returns
    with pytest.raises(ClerkAuthError):
        _authenticator(rsa_key).verify(token)


def test_protected_route_requires_token(rsa_key: rsa.RSAPrivateKey) -> None:
    app.dependency_overrides[get_authenticator] = lambda: _authenticator(rsa_key)
    try:
        client = TestClient(app)

        # No token -> 401.
        assert client.get("/ingestion/status").status_code == 401

        # Invalid token -> 401.
        bad = client.get("/ingestion/status", headers={"Authorization": "Bearer not-a-jwt"})
        assert bad.status_code == 401

        # Valid token -> 200 with the verified subject.
        token = _make_token(rsa_key, sub="user_ok", role="editor")
        ok = client.get("/ingestion/status", headers={"Authorization": f"Bearer {token}"})
        assert ok.status_code == 200
        assert ok.json() == {"status": "authenticated", "subject": "user_ok", "role": "editor"}
    finally:
        app.dependency_overrides.clear()


# --- Fail-closed configuration behavior (T-0003 hardening) ---


def test_authenticator_requires_issuer() -> None:
    """A configured JWKS URL must never bypass issuer validation: no issuer -> ValueError."""
    with pytest.raises(ValueError):
        ClerkAuthenticator(
            issuer=None,
            jwks_url="https://x.clerk.accounts.dev/.well-known/jwks.json",
        )
    with pytest.raises(ValueError):
        ClerkAuthenticator(issuer="   ", jwks_url="https://x/.well-known/jwks.json")


def test_authorized_parties_absent_defaults_to_local(monkeypatch: pytest.MonkeyPatch) -> None:
    """Absent CLERK_AUTHORIZED_PARTIES -> local dev origin only (never unrestricted)."""
    monkeypatch.delenv("CLERK_AUTHORIZED_PARTIES", raising=False)
    assert _authorized_parties_from_env() == [DEV_DEFAULT_AZP]


def test_authorized_parties_blank_fails_closed(monkeypatch: pytest.MonkeyPatch) -> None:
    """Present-but-blank CLERK_AUTHORIZED_PARTIES -> ClerkConfigError (never unrestricted)."""
    for blank in ("", "   ", " , , ", ","):
        monkeypatch.setenv("CLERK_AUTHORIZED_PARTIES", blank)
        with pytest.raises(ClerkConfigError):
            _authorized_parties_from_env()


def _clear_authenticator_cache() -> None:
    get_authenticator.cache_clear()


def test_get_authenticator_none_without_issuer(monkeypatch: pytest.MonkeyPatch) -> None:
    """Missing/blank issuer -> no authenticator (protected routes will fail closed)."""
    monkeypatch.setenv("CLERK_AUTHORIZED_PARTIES", DEV_DEFAULT_AZP)
    for value, present in ((None, False), ("", True), ("   ", True)):
        if present:
            monkeypatch.setenv("CLERK_JWT_ISSUER", value)  # type: ignore[arg-type]
        else:
            monkeypatch.delenv("CLERK_JWT_ISSUER", raising=False)
        _clear_authenticator_cache()
        assert get_authenticator() is None
    _clear_authenticator_cache()


def test_get_authenticator_none_with_blank_authorized_parties(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Explicitly blank authorized parties -> no authenticator (fail closed)."""
    monkeypatch.setenv("CLERK_JWT_ISSUER", ISSUER)
    monkeypatch.setenv("CLERK_JWKS_URL", ISSUER + "/.well-known/jwks.json")
    monkeypatch.setenv("CLERK_AUTHORIZED_PARTIES", "   ")
    _clear_authenticator_cache()
    try:
        assert get_authenticator() is None
    finally:
        _clear_authenticator_cache()


def test_protected_route_503_when_unconfigured(monkeypatch: pytest.MonkeyPatch) -> None:
    """With auth unconfigured (no issuer), the protected route fails closed with a generic 503.

    The response must not leak configuration details or secrets.
    """
    monkeypatch.delenv("CLERK_JWT_ISSUER", raising=False)
    _clear_authenticator_cache()
    app.dependency_overrides.clear()
    try:
        client = TestClient(app)
        resp = client.get(
            "/ingestion/status", headers={"Authorization": "Bearer whatever"}
        )
        assert resp.status_code == 503
        assert resp.json() == {"detail": "authentication is not configured"}
    finally:
        _clear_authenticator_cache()
