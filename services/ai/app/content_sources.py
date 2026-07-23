"""Content source contract shared by the ingestion gate.

Mirrors services/core/internal/contentsource.Source and packages/shared ContentSource:
metadata only, never the source material itself.
"""

from enum import Enum

from pydantic import BaseModel


class ContentSourceStatus(str, Enum):
    PENDING = "pending"
    APPROVED = "approved"
    REJECTED = "rejected"
    EXPIRED = "expired"


class ContentSource(BaseModel):
    id: str
    title: str
    status: ContentSourceStatus
