"""Deterministic fake Sonnet provider for tests (T-0033).

Never calls Anthropic. Every response is a pure function of the request (or a
pre-scripted queue), so the same request always produces the same result —
required for reproducible job-lifecycle and quality-gate tests.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, List

from .schemas import CriterionResult, ProviderUsage, SonnetRequest, SonnetResult

ResponseFn = Callable[[SonnetRequest], SonnetResult]


def default_response(request: SonnetRequest) -> SonnetResult:
    """One criterion result per rubric criterion, full marks, high confidence."""
    return SonnetResult(
        model="fake-sonnet-v1",
        confidence=0.95,
        criteria=tuple(
            CriterionResult(
                criterion_id=c.criterion_id,
                marks_awarded=c.max_marks,
                feedback=f"Criterion {c.criterion_id} fully met.",
            )
            for c in request.rubric_criteria
        ),
        usage=ProviderUsage(input_tokens=100, output_tokens=50, cost_usd_micros=1000),
    )


@dataclass
class FakeSonnetProvider:
    """Test double satisfying SonnetProvider.

    Defaults to ``default_response``; pass a custom ``response_fn`` or a
    ``responses`` queue to script specific scenarios (low confidence, missing
    criteria, over-max marks, provider errors, etc).
    """

    response_fn: ResponseFn = default_response
    responses: List[SonnetResult] = field(default_factory=list)
    calls: List[SonnetRequest] = field(default_factory=list)

    def generate(self, request: SonnetRequest) -> SonnetResult:
        self.calls.append(request)
        if self.responses:
            return self.responses.pop(0)
        return self.response_fn(request)
