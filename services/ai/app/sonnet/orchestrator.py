"""Sonnet job orchestrator: retry/withhold policy over the provider and
quality gate (T-0033).

Fully automated — a withheld job stays withheld. There is no human-reviewer
routing: per docs/mvp-execution-plan.md, invalid or low-confidence Sonnet
output is withheld or retried automatically, never fabricated and never
queued for manual marking.
"""

from __future__ import annotations

from . import quality_gate
from .jobs import Job, JobAttempt, JobStatus, JobStore, now_iso
from .provider import ProviderError, SonnetProvider
from .schemas import SonnetRequest, SonnetResult

MAX_ATTEMPTS = 3


def _attempt_from_result(
    attempt_number: int, result: SonnetResult, decision: quality_gate.GateDecision
) -> JobAttempt:
    return JobAttempt(
        attempt_number=attempt_number,
        model=result.model,
        cost_usd_micros=result.usage.cost_usd_micros,
        confidence=result.confidence,
        verdict=decision.verdict.value,
        reasons=decision.reasons,
        created_at=now_iso(),
    )


def run_job(
    request: SonnetRequest,
    *,
    store: JobStore,
    provider: SonnetProvider,
    owner_subject: str,
    max_attempts: int = MAX_ATTEMPTS,
) -> Job:
    """Run one request through provider -> quality gate -> retry/withhold.

    Returns the job in its terminal state (SUCCEEDED or WITHHELD). A
    provider error counts as a retryable attempt; running out of attempts
    withholds rather than raising, so a caller can always inspect the trace.
    Raises jobs.JobIdConflict if request.job_id already exists.
    """
    job = store.create(request, owner_subject=owner_subject)
    previous_result: SonnetResult | None = None

    for attempt_number in range(1, max_attempts + 1):
        try:
            result = provider.generate(request)
        except ProviderError as exc:
            attempt = JobAttempt(
                attempt_number=attempt_number,
                model="unknown",
                cost_usd_micros=0,
                confidence=0.0,
                verdict=quality_gate.GateVerdict.RETRY.value,
                reasons=(f"provider error: {exc}",),
                created_at=now_iso(),
            )
            job = store.append_attempt(
                job.job_id, status=JobStatus.RETRYING, result=None, attempt=attempt
            )
            continue

        decision = quality_gate.evaluate(result, request.rubric_criteria)

        if decision.verdict == quality_gate.GateVerdict.ACCEPT and previous_result is not None:
            decision = quality_gate.check_consistency(previous_result, result)

        attempt = _attempt_from_result(attempt_number, result, decision)

        if decision.verdict == quality_gate.GateVerdict.ACCEPT:
            return store.append_attempt(
                job.job_id, status=JobStatus.SUCCEEDED, result=result, attempt=attempt
            )

        if decision.verdict == quality_gate.GateVerdict.WITHHOLD:
            return store.append_attempt(
                job.job_id, status=JobStatus.WITHHELD, result=None, attempt=attempt
            )

        previous_result = result
        job = store.append_attempt(
            job.job_id, status=JobStatus.RETRYING, result=None, attempt=attempt
        )

    return store.append_attempt(
        job.job_id,
        status=JobStatus.WITHHELD,
        result=None,
        attempt=JobAttempt(
            attempt_number=max_attempts + 1,
            model="n/a",
            cost_usd_micros=0,
            confidence=0.0,
            verdict=quality_gate.GateVerdict.WITHHOLD.value,
            reasons=("retry attempts exhausted",),
            created_at=now_iso(),
        ),
    )
