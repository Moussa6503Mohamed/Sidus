from __future__ import annotations

import datetime as dt
from pathlib import Path

import jwt
import pytest
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi.testclient import TestClient

from app.auth import ClerkAuthenticator, get_authenticator
from app.main import app
from app.sonnet.fake_provider import FakeSonnetProvider, default_response
from app.sonnet.jobs import SqliteJobStore
from app.sonnet.provider import get_provider
from app.sonnet.routes import get_job_store

ISSUER = "https://example.clerk.accounts.dev"
AZP = "http://localhost:3000"

VALID_PAYLOAD = {
    "job_id": "job-route-1",
    "task_type": "marking",
    "question_id": "q-1",
    "syllabus_id": "0610",
    "rubric_version_id": "rv-1",
    "language": "en",
    "explanation_version": "v1",
    "rubric_criteria": [{"criterion_id": "c1", "max_marks": 2}],
    "prompt_content_ref": "ref-1",
}


@pytest.fixture(scope="module")
def rsa_key() -> rsa.RSAPrivateKey:
    return rsa.generate_private_key(public_exponent=65537, key_size=2048)


def _token(private_key: rsa.RSAPrivateKey, *, sub: str = "user_123") -> str:
    now = dt.datetime.now(tz=dt.timezone.utc)
    payload = {
        "sub": sub,
        "iss": ISSUER,
        "azp": AZP,
        "sidus_role": "editor",
        "iat": now,
        "exp": now + dt.timedelta(seconds=300),
    }
    return jwt.encode(payload, private_key, algorithm="RS256", headers={"kid": "test-key"})


def _auth_headers(rsa_key: rsa.RSAPrivateKey) -> dict[str, str]:
    return {"Authorization": f"Bearer {_token(rsa_key)}"}


@pytest.fixture(autouse=True)
def _configured_authenticator(rsa_key: rsa.RSAPrivateKey):
    public_key = rsa_key.public_key()
    app.dependency_overrides[get_authenticator] = lambda: ClerkAuthenticator(
        issuer=ISSUER,
        authorized_parties=[AZP],
        key_resolver=lambda _token: public_key,
    )
    yield
    app.dependency_overrides.clear()


@pytest.fixture(autouse=True)
def _default_job_store(tmp_path: Path):
    """Never let a test fall through to the real default store path on disk."""
    app.dependency_overrides[get_job_store] = lambda: SqliteJobStore(
        str(tmp_path / "default-jobs.sqlite3")
    )


def test_submit_job_requires_authentication() -> None:
    client = TestClient(app)
    resp = client.post("/sonnet/jobs", json=VALID_PAYLOAD)
    assert resp.status_code == 401


def test_get_job_requires_authentication() -> None:
    client = TestClient(app)
    resp = client.get("/sonnet/jobs/does-not-exist")
    assert resp.status_code == 401


def test_submit_job_fails_closed_without_provider(rsa_key: rsa.RSAPrivateKey) -> None:
    client = TestClient(app)
    resp = client.post("/sonnet/jobs", json=VALID_PAYLOAD, headers=_auth_headers(rsa_key))
    assert resp.status_code == 503
    assert resp.json() == {"detail": "Sonnet provider is not configured"}


def test_submit_job_rejects_malformed_request(rsa_key: rsa.RSAPrivateKey) -> None:
    client = TestClient(app)
    resp = client.post(
        "/sonnet/jobs",
        json={**VALID_PAYLOAD, "unexpected": "field"},
        headers=_auth_headers(rsa_key),
    )
    assert resp.status_code == 422


def test_submit_and_fetch_job_happy_path(rsa_key: rsa.RSAPrivateKey, tmp_path: Path) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    app.dependency_overrides[get_job_store] = lambda: store
    app.dependency_overrides[get_provider] = lambda: FakeSonnetProvider(
        response_fn=default_response
    )
    client = TestClient(app)

    submit_resp = client.post(
        "/sonnet/jobs", json=VALID_PAYLOAD, headers=_auth_headers(rsa_key)
    )
    assert submit_resp.status_code == 200
    body = submit_resp.json()
    assert body["status"] == "succeeded"
    assert body["result"]["adapter_version"] == "sonnet-adapter-v1"
    assert len(body["attempts"]) == 1
    assert body["attempts"][0]["verdict"] == "accept"

    fetch_resp = client.get(
        f"/sonnet/jobs/{VALID_PAYLOAD['job_id']}", headers=_auth_headers(rsa_key)
    )
    assert fetch_resp.status_code == 200
    assert fetch_resp.json() == body


def test_get_unknown_job_returns_404(rsa_key: rsa.RSAPrivateKey, tmp_path: Path) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    app.dependency_overrides[get_job_store] = lambda: store
    client = TestClient(app)

    resp = client.get("/sonnet/jobs/does-not-exist", headers=_auth_headers(rsa_key))
    assert resp.status_code == 404


def test_get_job_owned_by_another_principal_returns_404(
    rsa_key: rsa.RSAPrivateKey, tmp_path: Path
) -> None:
    """A job never leaks to a caller who isn't its owner, and existence itself
    doesn't leak — the response must be identical to an unknown job_id."""
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    app.dependency_overrides[get_job_store] = lambda: store
    app.dependency_overrides[get_provider] = lambda: FakeSonnetProvider(
        response_fn=default_response
    )
    client = TestClient(app)

    owner_headers = {"Authorization": f"Bearer {_token(rsa_key, sub='owner')}"}
    other_headers = {"Authorization": f"Bearer {_token(rsa_key, sub='someone-else')}"}

    submit_resp = client.post("/sonnet/jobs", json=VALID_PAYLOAD, headers=owner_headers)
    assert submit_resp.status_code == 200

    other_resp = client.get(
        f"/sonnet/jobs/{VALID_PAYLOAD['job_id']}", headers=other_headers
    )
    unknown_resp = client.get("/sonnet/jobs/does-not-exist", headers=other_headers)

    assert other_resp.status_code == 404
    assert other_resp.json() == unknown_resp.json()

    owner_resp = client.get(f"/sonnet/jobs/{VALID_PAYLOAD['job_id']}", headers=owner_headers)
    assert owner_resp.status_code == 200


def test_submit_job_with_duplicate_job_id_returns_409(
    rsa_key: rsa.RSAPrivateKey, tmp_path: Path
) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    app.dependency_overrides[get_job_store] = lambda: store
    app.dependency_overrides[get_provider] = lambda: FakeSonnetProvider(
        response_fn=default_response
    )
    client = TestClient(app)

    first = client.post("/sonnet/jobs", json=VALID_PAYLOAD, headers=_auth_headers(rsa_key))
    assert first.status_code == 200

    second = client.post("/sonnet/jobs", json=VALID_PAYLOAD, headers=_auth_headers(rsa_key))
    assert second.status_code == 409


def test_service_marking_route_is_token_gated_and_fake_provider_only(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    monkeypatch.setenv("SIDUS_CORE_SERVICE_TOKEN", "service-test-token")
    store = SqliteJobStore(str(tmp_path / "service-jobs.sqlite3"))
    app.dependency_overrides[get_job_store] = lambda: store
    app.dependency_overrides[get_provider] = lambda: FakeSonnetProvider(response_fn=default_response)
    payload = {
        "jobId": "service-marking-1", "attemptId": "opaque-attempt", "questionId": "opaque-question",
        "syllabusId": "opaque-syllabus", "rubricVersionId": "opaque-rubric",
        "rubricCriteria": [{"criterionId": "c1", "maxMarks": 2}], "promptContentRef": "opaque-attempt",
    }
    client = TestClient(app)
    assert client.post("/sonnet/marking-jobs", json=payload).status_code == 401
    response = client.post("/sonnet/marking-jobs", json=payload, headers={"Authorization": "Bearer service-test-token"})
    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "accepted"
    assert body["result"]["awardedMarks"] == 2
