"""Strict Sonnet adapter contracts: request, structured result, and provider trace.

Fields mirror CLAUDE.md's canonical cache key (question + syllabus + rubric +
language + explanation version) plus the version/cost/confidence trace T-0033
requires for the automated quality gate. Every model forbids unknown fields —
a provider response with an unexpected shape fails validation rather than
silently passing through. Nothing here ever carries raw source text, PDFs, or
extracted content: only opaque identifiers and model-produced structured
output.
"""

from __future__ import annotations

from enum import Enum
from typing import Any, Literal
from urllib.parse import quote

from pydantic import BaseModel, ConfigDict, Field, model_validator

ADAPTER_VERSION: str = "sonnet-adapter-v1"


class SonnetTaskType(str, Enum):
    REVIEW = "review"
    MARKING = "marking"


class RubricCriterion(BaseModel):
    model_config = ConfigDict(extra="forbid")

    criterion_id: str = Field(min_length=1, max_length=64)
    max_marks: int = Field(ge=1, le=100)
    descriptor: str = Field(default="", max_length=4000)

class PrivateMarkingContext(BaseModel):
    """Private service-only context for a written marking call.

    It is accepted only on the service route, omitted from all public job projections, and is
    available to a future provider implementation without a browser-facing fetch path.
    """
    model_config = ConfigDict(extra="forbid")
    answer: dict[str, Any] = Field(min_length=1)


class SonnetRequest(BaseModel):
    """One Sonnet call request.

    ``prompt_content_ref`` is an opaque pointer the caller resolves elsewhere;
    this contract never carries or resolves source content itself.
    """

    model_config = ConfigDict(extra="forbid")

    job_id: str = Field(min_length=1, max_length=128)
    task_type: SonnetTaskType
    question_id: str = Field(min_length=1, max_length=128)
    syllabus_id: str = Field(min_length=1, max_length=128)
    rubric_version_id: str = Field(min_length=1, max_length=128)
    language: str = Field(min_length=2, max_length=16)
    explanation_version: str = Field(min_length=1, max_length=32)
    rubric_criteria: tuple[RubricCriterion, ...] = Field(min_length=1, max_length=64)
    prompt_content_ref: str = Field(min_length=1, max_length=256)
    private_marking_context: PrivateMarkingContext | None = None

    @model_validator(mode="after")
    def _unique_criteria(self) -> "SonnetRequest":
        ids = [c.criterion_id for c in self.rubric_criteria]
        if len(ids) != len(set(ids)):
            raise ValueError("rubric_criteria ids must be unique")
        return self

    def cache_key(self) -> str:
        """The canonical explanation cache key defined in CLAUDE.md.

        Each field is percent-encoded before joining so a "|" occurring
        inside a field (e.g. a syllabus id) can never be mistaken for the
        separator and collide with a different field split.
        """
        parts = [
            self.question_id,
            self.syllabus_id,
            self.rubric_version_id,
            self.language,
            self.explanation_version,
        ]
        return "|".join(quote(part, safe="") for part in parts)


class CriterionResult(BaseModel):
    model_config = ConfigDict(extra="forbid")

    criterion_id: str = Field(min_length=1, max_length=64)
    marks_awarded: int = Field(ge=0)
    feedback: str = Field(min_length=1, max_length=2000)


class ProviderUsage(BaseModel):
    model_config = ConfigDict(extra="forbid")

    input_tokens: int = Field(ge=0)
    output_tokens: int = Field(ge=0)
    cost_usd_micros: int = Field(ge=0)


class SonnetResult(BaseModel):
    """Strict structured provider output. Every field is validated; nothing
    beyond this shape is ever trusted or persisted."""

    model_config = ConfigDict(extra="forbid")

    schema_version: Literal["v1"] = "v1"
    adapter_version: Literal["sonnet-adapter-v1"] = ADAPTER_VERSION
    model: str = Field(min_length=1, max_length=128)
    confidence: float = Field(ge=0, le=1)
    criteria: tuple[CriterionResult, ...] = Field(min_length=1, max_length=64)
    usage: ProviderUsage

    @model_validator(mode="after")
    def _unique_criteria(self) -> "SonnetResult":
        ids = [c.criterion_id for c in self.criteria]
        if len(ids) != len(set(ids)):
            raise ValueError("criteria ids must be unique")
        return self
