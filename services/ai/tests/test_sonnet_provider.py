from app.sonnet.fake_provider import FakeSonnetProvider, default_response
from app.sonnet.provider import get_provider
from app.sonnet.schemas import RubricCriterion, SonnetRequest, SonnetTaskType


def _request() -> SonnetRequest:
    return SonnetRequest(
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


def test_get_provider_is_always_none_fail_closed() -> None:
    assert get_provider() is None


def test_fake_provider_default_response_is_deterministic() -> None:
    provider = FakeSonnetProvider()
    request = _request()

    first = provider.generate(request)
    second = provider.generate(request)

    assert first == second
    assert first == default_response(request)
    assert provider.calls == [request, request]


def test_fake_provider_scripted_queue_is_consumed_in_order() -> None:
    request = _request()
    scripted = default_response(request)
    provider = FakeSonnetProvider(responses=[scripted])

    assert provider.generate(request) is scripted
    # Queue drained -> falls back to response_fn (default_response).
    assert provider.generate(request) == default_response(request)
