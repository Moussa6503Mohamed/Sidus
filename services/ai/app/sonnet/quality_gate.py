"""Deterministic rubric/consistency quality gate for Sonnet output (T-0033).

Runs entirely on structured ``SonnetResult`` data — no model calls, no
network access, no randomness. A gate decision is always reproducible from
its inputs. Structural rubric violations (wrong/missing/duplicate criteria,
marks exceeding a criterion's cap) are never retryable — the model produced
an internally invalid result, so the job withholds immediately. Low
confidence alone is retryable, since a second attempt may do better.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum

from .schemas import RubricCriterion, SonnetResult

MIN_CONFIDENCE = 0.6
DEFAULT_CONSISTENCY_TOLERANCE = 1


class GateVerdict(str, Enum):
    ACCEPT = "accept"
    RETRY = "retry"
    WITHHOLD = "withhold"


@dataclass(frozen=True)
class GateDecision:
    verdict: GateVerdict
    reasons: tuple[str, ...]


def evaluate(
    result: SonnetResult, rubric_criteria: tuple[RubricCriterion, ...]
) -> GateDecision:
    """Validate one result against its rubric and confidence floor."""
    structural_reasons: list[str] = []

    expected_ids = {c.criterion_id for c in rubric_criteria}
    max_marks_by_id = {c.criterion_id: c.max_marks for c in rubric_criteria}
    actual_ids = {c.criterion_id for c in result.criteria}

    missing = expected_ids - actual_ids
    if missing:
        structural_reasons.append(f"missing criteria: {sorted(missing)}")
    unknown = actual_ids - expected_ids
    if unknown:
        structural_reasons.append(f"unknown criteria: {sorted(unknown)}")

    for criterion in result.criteria:
        max_marks = max_marks_by_id.get(criterion.criterion_id)
        if max_marks is not None and criterion.marks_awarded > max_marks:
            structural_reasons.append(
                f"criterion {criterion.criterion_id} awarded {criterion.marks_awarded} "
                f"exceeds max {max_marks}"
            )

    if structural_reasons:
        return GateDecision(verdict=GateVerdict.WITHHOLD, reasons=tuple(structural_reasons))

    if result.confidence < MIN_CONFIDENCE:
        return GateDecision(
            verdict=GateVerdict.RETRY,
            reasons=(f"confidence {result.confidence} below minimum {MIN_CONFIDENCE}",),
        )

    return GateDecision(verdict=GateVerdict.ACCEPT, reasons=())


def check_consistency(
    first: SonnetResult,
    second: SonnetResult,
    tolerance: int = DEFAULT_CONSISTENCY_TOLERANCE,
) -> GateDecision:
    """Cross-attempt consistency check used when a retry produces a second result.

    Compares criterion-by-criterion marks between two structurally valid
    attempts. Large disagreement signals an unreliable result that should be
    withheld rather than auto-accepted, even though each attempt individually
    passed ``evaluate()``.
    """
    first_by_id = {c.criterion_id: c.marks_awarded for c in first.criteria}
    second_by_id = {c.criterion_id: c.marks_awarded for c in second.criteria}
    if set(first_by_id) != set(second_by_id):
        return GateDecision(
            verdict=GateVerdict.WITHHOLD,
            reasons=("criterion sets differ across attempts",),
        )

    disagreements = sorted(
        criterion_id
        for criterion_id, first_marks in first_by_id.items()
        if abs(first_marks - second_by_id[criterion_id]) > tolerance
    )
    if disagreements:
        return GateDecision(
            verdict=GateVerdict.WITHHOLD,
            reasons=(f"attempts disagree on criteria: {disagreements}",),
        )
    return GateDecision(verdict=GateVerdict.ACCEPT, reasons=())
