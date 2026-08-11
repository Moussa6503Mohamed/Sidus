from pathlib import Path

import pytest

from app.sonnet.fake_provider import FakeSonnetProvider, default_response
from app.sonnet.jobs import JobIdConflict, JobStatus, SqliteJobStore
from app.sonnet.orchestrator import run_job
from app.sonnet.provider import ProviderError
from app.sonnet.schemas import (
    CriterionResult,
    ProviderUsage,
    RubricCriterion,
    SonnetRequest,
    SonnetResult,
    SonnetTaskType,
)

OWNER = "user-1"


@pytest.fixture
def store(tmp_path: Path) -> SqliteJobStore:
    return SqliteJobStore(str(tmp_path / "jobs.sqlite3"))


def _request(job_id: str = "job-1") -> SonnetRequest:
    return SonnetRequest(
        job_id=job_id,
        task_type=SonnetTaskType.MARKING,
        question_id="q-1",
        syllabus_id="0610",
        rubric_version_id="rv-1",
        language="en",
        explanation_version="v1",
        rubric_criteria=(RubricCriterion(criterion_id="c1", max_marks=2),),
        prompt_content_ref="ref-1",
    )


def _valid_result(confidence: float = 0.9) -> SonnetResult:
    return SonnetResult(
        model="sonnet-test",
        confidence=confidence,
        criteria=(CriterionResult(criterion_id="c1", marks_awarded=2, feedback="ok"),),
        usage=ProviderUsage(input_tokens=1, output_tokens=1, cost_usd_micros=1),
    )


def _structurally_invalid_result() -> SonnetResult:
    return SonnetResult(
        model="sonnet-test",
        confidence=0.99,
        criteria=(CriterionResult(criterion_id="c-bogus", marks_awarded=1, feedback="ok"),),
        usage=ProviderUsage(input_tokens=1, output_tokens=1, cost_usd_micros=1),
    )


def test_accepts_and_persists_result_on_first_valid_attempt(store: SqliteJobStore) -> None:
    request = _request()
    provider = FakeSonnetProvider(response_fn=default_response)

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER)

    assert job.status == JobStatus.SUCCEEDED
    assert job.owner_subject == OWNER
    assert job.result == default_response(request)
    assert len(job.attempts) == 1
    assert job.attempts[0].verdict == "accept"


def test_withholds_immediately_on_structural_violation(store: SqliteJobStore) -> None:
    request = _request()
    provider = FakeSonnetProvider(responses=[_structurally_invalid_result()])

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER)

    assert job.status == JobStatus.WITHHELD
    assert job.result is None
    assert len(job.attempts) == 1
    assert len(provider.calls) == 1  # never retried


def test_retries_low_confidence_then_accepts_on_agreement(store: SqliteJobStore) -> None:
    request = _request()
    low = _valid_result(confidence=0.1)
    high = _valid_result(confidence=0.9)
    provider = FakeSonnetProvider(responses=[low, high])

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER)

    assert job.status == JobStatus.SUCCEEDED
    assert job.result == high
    assert [a.verdict for a in job.attempts] == ["retry", "accept"]
    assert len(provider.calls) == 2


def test_withholds_when_retry_attempts_disagree(store: SqliteJobStore) -> None:
    request = _request()
    low_confidence = _valid_result(confidence=0.1)
    disagreeing = SonnetResult(
        model="sonnet-test",
        confidence=0.9,
        criteria=(CriterionResult(criterion_id="c1", marks_awarded=0, feedback="disagrees"),),
        usage=ProviderUsage(input_tokens=1, output_tokens=1, cost_usd_micros=1),
    )
    provider = FakeSonnetProvider(responses=[low_confidence, disagreeing])

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER)

    assert job.status == JobStatus.WITHHELD
    assert job.result is None
    assert job.attempts[-1].verdict == "withhold"
    assert any("disagree" in r for r in job.attempts[-1].reasons)


def test_withholds_after_exhausting_retries_on_persistent_low_confidence(
    store: SqliteJobStore,
) -> None:
    request = _request()
    provider = FakeSonnetProvider(response_fn=lambda req: _valid_result(confidence=0.1))

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER, max_attempts=3)

    assert job.status == JobStatus.WITHHELD
    assert job.result is None
    # 3 real attempts (all retry) + 1 terminal exhaustion record.
    assert len(job.attempts) == 4
    assert job.attempts[-1].reasons == ("retry attempts exhausted",)
    assert len(provider.calls) == 3


def test_provider_error_is_recorded_as_retry_and_can_still_succeed(store: SqliteJobStore) -> None:
    request = _request()
    calls = {"count": 0}

    def flaky(req: SonnetRequest) -> SonnetResult:
        calls["count"] += 1
        if calls["count"] == 1:
            raise ProviderError("temporary upstream failure")
        return default_response(req)

    provider = FakeSonnetProvider(response_fn=flaky)

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER)

    assert job.status == JobStatus.SUCCEEDED
    assert job.attempts[0].verdict == "retry"
    assert "provider error" in job.attempts[0].reasons[0]
    assert job.attempts[1].verdict == "accept"


def test_never_fabricates_a_result_when_all_attempts_fail(store: SqliteJobStore) -> None:
    request = _request()
    provider = FakeSonnetProvider(
        response_fn=lambda req: (_ for _ in ()).throw(ProviderError("always down"))
    )

    job = run_job(request, store=store, provider=provider, owner_subject=OWNER, max_attempts=2)

    assert job.status == JobStatus.WITHHELD
    assert job.result is None


def test_duplicate_job_id_raises_conflict_instead_of_crashing(store: SqliteJobStore) -> None:
    request = _request()
    provider = FakeSonnetProvider(response_fn=default_response)
    run_job(request, store=store, provider=provider, owner_subject=OWNER)

    with pytest.raises(JobIdConflict):
        run_job(request, store=store, provider=provider, owner_subject="user-2")
