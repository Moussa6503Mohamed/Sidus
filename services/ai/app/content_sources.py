"""Content source contract shared by the ingestion gate.

Mirrors services/core/internal/contentsource.Source and packages/shared ContentSource:
metadata only, never the source material itself.
"""

from datetime import datetime
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


class ContentSourceEventType(str, Enum):
    METADATA_UPDATED = "metadata_updated"


class ContentSourceEvent(BaseModel):
    """Immutable audit record of a metadata change (T-0002).

    Mirrors packages/shared ContentSourceEvent and the Go Event: field names only, never
    the field values, and never any source material. Included for contract parity; the
    ingestion gate does not consume events.
    """

    id: str
    content_source_id: str
    event_type: ContentSourceEventType
    actor_id: str
    event_time: datetime
    changed_fields: list[str]
