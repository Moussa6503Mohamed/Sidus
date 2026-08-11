"""Sonnet-ready private-upload review contract.

This module is a schema boundary only. It does not fetch files, call Anthropic, OCR PDFs, or
persist model output. Future T-0032 passes only opaque upload/source ids through this contract.
"""
from __future__ import annotations
from typing import Literal
from pydantic import BaseModel, ConfigDict, Field

class PrivateReviewJob(BaseModel):
    model_config = ConfigDict(extra="forbid")
    job_id: str = Field(min_length=1, max_length=128)
    upload_id: str = Field(min_length=1, max_length=128)
    content_source_id: str = Field(min_length=1, max_length=128)
    adapter_version: Literal["sonnet-review-v1"] = "sonnet-review-v1"

class PrivateReviewResultMetadata(BaseModel):
    model_config = ConfigDict(extra="forbid")
    """Metadata-only result. No extracted text, questions, or answer content allowed."""
    schema_version: Literal["v1"] = "v1"
    model: str = Field(min_length=1, max_length=128)
    confidence: float = Field(ge=0, le=1)
    requires_human_review: Literal[True] = True
