import pytest
from pydantic import ValidationError

from app.sonnet.schemas import (
    CriterionResult,
    ProviderUsage,
    RubricCriterion,
    SonnetRequest,
    SonnetResult,
    SonnetTaskType,
)


def _request(**overrides) -> SonnetRequest:
    fields = dict(
        job_id="job-1",
        task_type=SonnetTaskType.MARKING,
        question_id="q-1",
        syllabus_id="0610",
        rubric_version_id="rv-1",
        language="en",
        explanation_version="v1",
        rubric_criteria=(RubricCriterion(criterion_id="c1", max_marks=2),),
        prompt_content_ref="ref-1",
    )
    fields.update(overrides)
    return SonnetRequest(**fields)


def test_request_cache_key_matches_canonical_fields() -> None:
    request = _request()
    assert request.cache_key() == "q-1|0610|rv-1|en|v1"


def test_request_cache_key_has_no_delimiter_collision() -> None:
    """A '|' inside a field must never be mistaken for the field separator."""
    ambiguous_split = _request(question_id="a|b", syllabus_id="c")
    unambiguous_split = _request(question_id="a", syllabus_id="b|c")

    assert ambiguous_split.cache_key() != unambiguous_split.cache_key()


def test_request_rejects_unknown_fields() -> None:
    with pytest.raises(ValidationError):
        _request(extra_field="nope")


def test_request_rejects_duplicate_criterion_ids() -> None:
    with pytest.raises(ValidationError):
        _request(
            rubric_criteria=(
                RubricCriterion(criterion_id="c1", max_marks=2),
                RubricCriterion(criterion_id="c1", max_marks=3),
            )
        )


def test_request_rejects_empty_rubric_criteria() -> None:
    with pytest.raises(ValidationError):
        _request(rubric_criteria=())


def _result(**overrides) -> SonnetResult:
    fields = dict(
        model="sonnet-test",
        confidence=0.9,
        criteria=(CriterionResult(criterion_id="c1", marks_awarded=2, feedback="good"),),
        usage=ProviderUsage(input_tokens=10, output_tokens=5, cost_usd_micros=100),
    )
    fields.update(overrides)
    return SonnetResult(**fields)


def test_result_rejects_unknown_fields() -> None:
    with pytest.raises(ValidationError):
        _result(extracted_text="leaked source content")


def test_result_rejects_out_of_range_confidence() -> None:
    with pytest.raises(ValidationError):
        _result(confidence=1.5)


def test_result_rejects_duplicate_criteria() -> None:
    with pytest.raises(ValidationError):
        _result(
            criteria=(
                CriterionResult(criterion_id="c1", marks_awarded=1, feedback="a"),
                CriterionResult(criterion_id="c1", marks_awarded=2, feedback="b"),
            )
        )


def test_result_defaults_carry_schema_and_adapter_version() -> None:
    result = _result()
    assert result.schema_version == "v1"
    assert result.adapter_version == "sonnet-adapter-v1"
