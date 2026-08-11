from pathlib import Path

import pytest

from app.sonnet.jobs import JobAttempt, JobIdConflict, JobStatus, SqliteJobStore, now_iso
from app.sonnet.schemas import (
    CriterionResult,
    ProviderUsage,
    RubricCriterion,
    SonnetRequest,
    SonnetResult,
    SonnetTaskType,
)


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


def _result() -> SonnetResult:
    return SonnetResult(
        model="sonnet-test",
        confidence=0.9,
        criteria=(CriterionResult(criterion_id="c1", marks_awarded=2, feedback="ok"),),
        usage=ProviderUsage(input_tokens=1, output_tokens=1, cost_usd_micros=1),
    )


def test_create_and_get_round_trips_the_request(tmp_path: Path) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    request = _request()

    created = store.create(request, owner_subject="user-1")
    fetched = store.get(request.job_id)

    assert created.status == JobStatus.PENDING
    assert created.owner_subject == "user-1"
    assert fetched is not None
    assert fetched.request == request
    assert fetched.owner_subject == "user-1"
    assert fetched.attempts == ()


def test_get_returns_none_for_unknown_job(tmp_path: Path) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    assert store.get("does-not-exist") is None


def test_create_raises_conflict_on_duplicate_job_id(tmp_path: Path) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    request = _request()
    store.create(request, owner_subject="user-1")

    with pytest.raises(JobIdConflict) as exc_info:
        store.create(request, owner_subject="user-2")

    assert exc_info.value.job_id == request.job_id
    # The original job (and its owner) must survive the failed second create.
    job = store.get(request.job_id)
    assert job is not None
    assert job.owner_subject == "user-1"


def test_append_attempt_is_append_only_and_updates_status(tmp_path: Path) -> None:
    store = SqliteJobStore(str(tmp_path / "jobs.sqlite3"))
    request = _request()
    store.create(request, owner_subject="user-1")

    attempt_1 = JobAttempt(
        attempt_number=1,
        model="sonnet-test",
        cost_usd_micros=10,
        confidence=0.2,
        verdict="retry",
        reasons=("confidence too low",),
        created_at=now_iso(),
    )
    store.append_attempt(request.job_id, status=JobStatus.RETRYING, result=None, attempt=attempt_1)

    attempt_2 = JobAttempt(
        attempt_number=2,
        model="sonnet-test",
        cost_usd_micros=20,
        confidence=0.9,
        verdict="accept",
        reasons=(),
        created_at=now_iso(),
    )
    result = _result()
    job = store.append_attempt(
        request.job_id, status=JobStatus.SUCCEEDED, result=result, attempt=attempt_2
    )

    assert job.status == JobStatus.SUCCEEDED
    assert job.result == result
    assert [a.attempt_number for a in job.attempts] == [1, 2]
    assert job.attempts[0].verdict == "retry"
    assert job.attempts[1].verdict == "accept"


def test_durability_survives_reopening_the_store(tmp_path: Path) -> None:
    path = str(tmp_path / "jobs.sqlite3")
    request = _request()

    writer = SqliteJobStore(path)
    writer.create(request, owner_subject="user-1")
    writer.append_attempt(
        request.job_id,
        status=JobStatus.WITHHELD,
        result=None,
        attempt=JobAttempt(
            attempt_number=1,
            model="sonnet-test",
            cost_usd_micros=5,
            confidence=0.9,
            verdict="withhold",
            reasons=("missing criteria",),
            created_at=now_iso(),
        ),
    )

    reopened = SqliteJobStore(path)
    job = reopened.get(request.job_id)

    assert job is not None
    assert job.owner_subject == "user-1"
    assert job.status == JobStatus.WITHHELD
    assert len(job.attempts) == 1
    assert job.attempts[0].reasons == ("missing criteria",)
