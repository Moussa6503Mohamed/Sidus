"""AI ingestion foundation: the rights/provenance gate.

Every ingestion path must call guard_ingestion() before touching a source. It rejects
every source except one whose status is 'approved', and logs the source ID and the
rejection reason so rejections are auditable.
"""

import logging

from .content_sources import ContentSource, ContentSourceStatus

logger = logging.getLogger("sidus.ai.ingestion")


class IngestionRejected(Exception):
    """Raised when a source is not eligible for ingestion."""

    def __init__(self, source_id: str, reason: str) -> None:
        self.source_id = source_id
        self.reason = reason
        super().__init__(reason)


def guard_ingestion(source: ContentSource) -> ContentSource:
    """Return source if it may be ingested, else raise IngestionRejected.

    Only status == 'approved' passes. Every other status (pending, rejected, expired,
    and any future value) is rejected.
    """
    if source.status != ContentSourceStatus.APPROVED:
        reason = f"source status is '{source.status.value}', ingestion requires 'approved'"
        logger.warning(
            "ingestion_rejected source_id=%s reason=%s",
            source.id,
            reason,
            extra={"source_id": source.id, "reason": reason},
        )
        raise IngestionRejected(source.id, reason)
    return source
