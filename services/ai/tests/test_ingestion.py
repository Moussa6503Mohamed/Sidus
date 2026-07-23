import logging

import pytest

from app.content_sources import ContentSource, ContentSourceStatus
from app.ingestion import IngestionRejected, guard_ingestion


@pytest.mark.parametrize(
    "status",
    [ContentSourceStatus.PENDING, ContentSourceStatus.REJECTED, ContentSourceStatus.EXPIRED],
)
def test_guard_ingestion_rejects_every_non_approved_status(
    status: ContentSourceStatus, caplog: pytest.LogCaptureFixture
) -> None:
    source = ContentSource(id="src-1", title="Some syllabus", status=status)

    with caplog.at_level(logging.WARNING, logger="sidus.ai.ingestion"):
        with pytest.raises(IngestionRejected) as exc_info:
            guard_ingestion(source)

    assert exc_info.value.source_id == "src-1"
    assert status.value in exc_info.value.reason
    assert any(
        record.source_id == "src-1" and record.reason == exc_info.value.reason
        for record in caplog.records
    )


def test_guard_ingestion_allows_approved_source() -> None:
    source = ContentSource(id="src-2", title="Some syllabus", status=ContentSourceStatus.APPROVED)

    result = guard_ingestion(source)

    assert result is source
