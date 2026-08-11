from pydantic import ValidationError
import pytest
from app.review_intake import PrivateReviewJob, PrivateReviewResultMetadata

def test_review_contract_is_metadata_only() -> None:
    assert PrivateReviewJob(job_id="j", upload_id="u", content_source_id="s").adapter_version == "sonnet-review-v1"
    assert PrivateReviewResultMetadata(model="sonnet", confidence=0.8).requires_human_review is True

def test_review_contract_rejects_auto_publish_or_content_fields() -> None:
    with pytest.raises(ValidationError):
        PrivateReviewResultMetadata(model="sonnet", confidence=1, requires_human_review=False)
    with pytest.raises(ValidationError):
        PrivateReviewResultMetadata(model="sonnet", confidence=1, extracted_text="no")
