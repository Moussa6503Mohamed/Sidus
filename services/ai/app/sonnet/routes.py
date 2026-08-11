"""Sonnet adapter API surface (T-0033).

``POST /sonnet/jobs`` submits a request and runs it synchronously through the
provider + quality gate + retry/withhold pipeline; ``GET /sonnet/jobs/{id}``
returns the job's current state and attempt trace. Every route requires a
verified Clerk session, and every job is owner-scoped to the caller who
created it — a caller may only ever read their own jobs. When no provider is
configured the submit route fails closed with 503 — it never fabricates a
result.
"""

from __future__ import annotations

import os
from functools import lru_cache
from typing import Optional

from fastapi import APIRouter, Depends, Header, HTTPException, status
from pydantic import BaseModel, ConfigDict, Field

from ..auth import Principal, require_clerk_session
from .jobs import Job, JobIdConflict, JobStore, SqliteJobStore
from .orchestrator import run_job
from .provider import SonnetProvider, get_provider
from .schemas import SonnetRequest

router = APIRouter(prefix="/sonnet", tags=["sonnet"])

DEFAULT_JOB_STORE_PATH = "sonnet_jobs.sqlite3"
SERVICE_OWNER = "sidus-core-service"

def require_service_token(authorization: str | None = Header(default=None)) -> None:
    """Fail closed service-to-service authentication; never accepts a Clerk token here."""
    expected = os.getenv("SIDUS_CORE_SERVICE_TOKEN")
    if not expected or authorization != f"Bearer {expected}":
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="service authentication is required")

class MarkingCriterion(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)
    criterion_id: str = Field(alias="criterionId", min_length=1, max_length=64)
    max_marks: int = Field(alias="maxMarks", ge=1, le=100)
class MarkingJobRequest(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)
    job_id: str = Field(alias="jobId", min_length=1, max_length=128)
    attempt_id: str = Field(alias="attemptId", min_length=1, max_length=128)
    question_id: str = Field(alias="questionId", min_length=1, max_length=128)
    syllabus_id: str = Field(alias="syllabusId", min_length=1, max_length=128)
    rubric_version_id: str = Field(alias="rubricVersionId", min_length=1, max_length=128)
    rubric_criteria: tuple[MarkingCriterion, ...] = Field(alias="rubricCriteria", min_length=1, max_length=64)
    prompt_content_ref: str = Field(alias="promptContentRef", min_length=1, max_length=256)


@lru_cache
def get_job_store() -> JobStore:
    path = os.getenv("SONNET_JOB_STORE_PATH", DEFAULT_JOB_STORE_PATH)
    return SqliteJobStore(path)


def _job_to_dict(job: Job) -> dict:
    return {
        "job_id": job.job_id,
        "status": job.status.value,
        "result": job.result.model_dump() if job.result else None,
        "attempts": [
            {
                "attempt_number": a.attempt_number,
                "model": a.model,
                "cost_usd_micros": a.cost_usd_micros,
                "confidence": a.confidence,
                "verdict": a.verdict,
                "reasons": list(a.reasons),
                "created_at": a.created_at,
            }
            for a in job.attempts
        ],
        "created_at": job.created_at,
        "updated_at": job.updated_at,
    }


@router.post("/jobs")
def submit_job(
    request: SonnetRequest,
    principal: Principal = Depends(require_clerk_session),
    store: JobStore = Depends(get_job_store),
    provider: Optional[SonnetProvider] = Depends(get_provider),
) -> dict:
    if provider is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Sonnet provider is not configured",
        )
    try:
        job = run_job(request, store=store, provider=provider, owner_subject=principal.subject)
    except JobIdConflict as exc:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=f"job_id '{exc.job_id}' already exists",
        ) from exc
    return _job_to_dict(job)


@router.get("/jobs/{job_id}")
def get_job(
    job_id: str,
    principal: Principal = Depends(require_clerk_session),
    store: JobStore = Depends(get_job_store),
) -> dict:
    job = store.get(job_id)
    # A job owned by someone else reports 404, not 403: existence itself
    # must not leak to a caller who isn't the owner.
    if job is None or job.owner_subject != principal.subject:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="job not found")
    return _job_to_dict(job)

@router.post("/marking-jobs")
def submit_marking_job(
    request: MarkingJobRequest,
    _: None = Depends(require_service_token),
    store: JobStore = Depends(get_job_store),
    provider: Optional[SonnetProvider] = Depends(get_provider),
) -> dict:
    if provider is None:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Sonnet provider is not configured")
    adapter_request = SonnetRequest(
        job_id=request.job_id, task_type="marking", question_id=request.question_id,
        syllabus_id=request.syllabus_id, rubric_version_id=request.rubric_version_id,
        language="en", explanation_version="v1",
        rubric_criteria=tuple({"criterion_id": c.criterion_id, "max_marks": c.max_marks} for c in request.rubric_criteria),
        prompt_content_ref=request.prompt_content_ref,
    )
    try:
        job = run_job(adapter_request, store=store, provider=provider, owner_subject=SERVICE_OWNER)
    except JobIdConflict:
        existing = store.get(request.job_id)
        if existing is None or existing.owner_subject != SERVICE_OWNER:
            raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="marking job conflict")
        job = existing
    if job.status.value == "succeeded" and job.result is not None:
        result = job.result
        return {"status":"accepted", "result": {"criterionMarks":[{"criterionId":c.criterion_id,"marksAwarded":c.marks_awarded,"feedback":c.feedback} for c in result.criteria], "awardedMarks":sum(c.marks_awarded for c in result.criteria), "maxMarks":sum(c.max_marks for c in request.rubric_criteria), "model":result.model, "modelVersion":result.adapter_version, "costUsdMicros":result.usage.cost_usd_micros, "confidence":result.confidence}}
    return {"status":"withheld", "reason":"quality_gate_withheld"}
