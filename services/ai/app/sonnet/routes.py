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

from fastapi import APIRouter, Depends, HTTPException, status

from ..auth import Principal, require_clerk_session
from .jobs import Job, JobIdConflict, JobStore, SqliteJobStore
from .orchestrator import run_job
from .provider import SonnetProvider, get_provider
from .schemas import SonnetRequest

router = APIRouter(prefix="/sonnet", tags=["sonnet"])

DEFAULT_JOB_STORE_PATH = "sonnet_jobs.sqlite3"


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
