"""Durable Sonnet job lifecycle (T-0033).

A job tracks one Sonnet request through retry/withhold to a terminal state.
Every attempt (provider call + gate verdict) is recorded as an immutable
trace row: model, cost, confidence, verdict, reasons. ``SqliteJobStore``
persists to a local SQLite file so job state survives process restarts
without requiring a live database connection or any dependency beyond the
Python standard library.
"""

from __future__ import annotations

import json
import sqlite3
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import datetime, timezone
from enum import Enum
from typing import Iterator, Optional, Protocol

from .schemas import SonnetRequest, SonnetResult


class JobStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    SUCCEEDED = "succeeded"
    RETRYING = "retrying"
    WITHHELD = "withheld"
    FAILED = "failed"


@dataclass(frozen=True)
class JobAttempt:
    attempt_number: int
    model: str
    cost_usd_micros: int
    confidence: float
    verdict: str
    reasons: tuple[str, ...]
    created_at: str


@dataclass(frozen=True)
class Job:
    job_id: str
    owner_subject: str
    status: JobStatus
    request: SonnetRequest
    result: Optional[SonnetResult]
    attempts: tuple[JobAttempt, ...]
    created_at: str
    updated_at: str


class JobIdConflict(Exception):
    """Raised when a submitted job_id collides with an existing job.

    job_id is caller-supplied (schemas.SonnetRequest.job_id), so a retried
    submission or two callers picking the same id must surface as a
    structured conflict, not an unhandled database error.
    """

    def __init__(self, job_id: str) -> None:
        self.job_id = job_id
        super().__init__(f"job_id '{job_id}' already exists")


class JobStore(Protocol):
    def create(self, request: SonnetRequest, *, owner_subject: str) -> Job: ...

    def get(self, job_id: str) -> Optional[Job]: ...

    def append_attempt(
        self,
        job_id: str,
        *,
        status: JobStatus,
        result: Optional[SonnetResult],
        attempt: JobAttempt,
    ) -> Job: ...


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


class SqliteJobStore:
    """Durable JobStore backed by a local SQLite file.

    Attempts are append-only, matching T-0033's version/cost/confidence
    trace requirement: each ``append_attempt`` call adds one immutable row
    and never rewrites history.
    """

    def __init__(self, path: str) -> None:
        self._path = path
        with self._connect() as conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS jobs (
                    job_id TEXT PRIMARY KEY,
                    owner_subject TEXT NOT NULL,
                    status TEXT NOT NULL,
                    request_json TEXT NOT NULL,
                    result_json TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                )
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS job_attempts (
                    job_id TEXT NOT NULL,
                    attempt_number INTEGER NOT NULL,
                    model TEXT NOT NULL,
                    cost_usd_micros INTEGER NOT NULL,
                    confidence REAL NOT NULL,
                    verdict TEXT NOT NULL,
                    reasons_json TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    PRIMARY KEY (job_id, attempt_number)
                )
                """
            )

    @contextmanager
    def _connect(self) -> Iterator[sqlite3.Connection]:
        conn = sqlite3.connect(self._path)
        try:
            yield conn
            conn.commit()
        finally:
            conn.close()

    def create(self, request: SonnetRequest, *, owner_subject: str) -> Job:
        now = now_iso()
        try:
            with self._connect() as conn:
                conn.execute(
                    "INSERT INTO jobs "
                    "(job_id, owner_subject, status, request_json, result_json, created_at, updated_at) "
                    "VALUES (?, ?, ?, ?, NULL, ?, ?)",
                    (
                        request.job_id,
                        owner_subject,
                        JobStatus.PENDING.value,
                        request.model_dump_json(),
                        now,
                        now,
                    ),
                )
        except sqlite3.IntegrityError as exc:
            raise JobIdConflict(request.job_id) from exc
        return Job(
            job_id=request.job_id,
            owner_subject=owner_subject,
            status=JobStatus.PENDING,
            request=request,
            result=None,
            attempts=(),
            created_at=now,
            updated_at=now,
        )

    def get(self, job_id: str) -> Optional[Job]:
        with self._connect() as conn:
            row = conn.execute(
                "SELECT owner_subject, status, request_json, result_json, created_at, updated_at "
                "FROM jobs WHERE job_id = ?",
                (job_id,),
            ).fetchone()
            if row is None:
                return None
            owner_subject, status, request_json, result_json, created_at, updated_at = row
            attempt_rows = conn.execute(
                "SELECT attempt_number, model, cost_usd_micros, confidence, verdict, "
                "reasons_json, created_at FROM job_attempts WHERE job_id = ? "
                "ORDER BY attempt_number",
                (job_id,),
            ).fetchall()

        request = SonnetRequest.model_validate_json(request_json)
        result = SonnetResult.model_validate_json(result_json) if result_json else None
        attempts = tuple(
            JobAttempt(
                attempt_number=r[0],
                model=r[1],
                cost_usd_micros=r[2],
                confidence=r[3],
                verdict=r[4],
                reasons=tuple(json.loads(r[5])),
                created_at=r[6],
            )
            for r in attempt_rows
        )
        return Job(
            job_id=job_id,
            owner_subject=owner_subject,
            status=JobStatus(status),
            request=request,
            result=result,
            attempts=attempts,
            created_at=created_at,
            updated_at=updated_at,
        )

    def append_attempt(
        self,
        job_id: str,
        *,
        status: JobStatus,
        result: Optional[SonnetResult],
        attempt: JobAttempt,
    ) -> Job:
        now = now_iso()
        with self._connect() as conn:
            conn.execute(
                "INSERT INTO job_attempts "
                "(job_id, attempt_number, model, cost_usd_micros, confidence, verdict, "
                "reasons_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
                (
                    job_id,
                    attempt.attempt_number,
                    attempt.model,
                    attempt.cost_usd_micros,
                    attempt.confidence,
                    attempt.verdict,
                    json.dumps(list(attempt.reasons)),
                    attempt.created_at,
                ),
            )
            conn.execute(
                "UPDATE jobs SET status = ?, result_json = ?, updated_at = ? WHERE job_id = ?",
                (status.value, result.model_dump_json() if result else None, now, job_id),
            )

        job = self.get(job_id)
        if job is None:
            raise RuntimeError(f"job {job_id} vanished after its own write")
        return job
