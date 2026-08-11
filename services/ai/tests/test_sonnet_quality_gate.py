from app.sonnet.quality_gate import GateVerdict, check_consistency, evaluate
from app.sonnet.schemas import CriterionResult, ProviderUsage, RubricCriterion, SonnetResult

RUBRIC = (
    RubricCriterion(criterion_id="c1", max_marks=2),
    RubricCriterion(criterion_id="c2", max_marks=3),
)


def _result(criteria, confidence=0.9) -> SonnetResult:
    return SonnetResult(
        model="sonnet-test",
        confidence=confidence,
        criteria=criteria,
        usage=ProviderUsage(input_tokens=1, output_tokens=1, cost_usd_micros=1),
    )


def _full_marks_result(confidence=0.9) -> SonnetResult:
    return _result(
        criteria=(
            CriterionResult(criterion_id="c1", marks_awarded=2, feedback="ok"),
            CriterionResult(criterion_id="c2", marks_awarded=3, feedback="ok"),
        ),
        confidence=confidence,
    )


def test_accepts_complete_high_confidence_result() -> None:
    decision = evaluate(_full_marks_result(), RUBRIC)
    assert decision.verdict == GateVerdict.ACCEPT
    assert decision.reasons == ()


def test_withholds_on_missing_criterion() -> None:
    result = _result(criteria=(CriterionResult(criterion_id="c1", marks_awarded=2, feedback="ok"),))
    decision = evaluate(result, RUBRIC)
    assert decision.verdict == GateVerdict.WITHHOLD
    assert any("missing criteria" in r for r in decision.reasons)


def test_withholds_on_unknown_criterion() -> None:
    result = _result(
        criteria=(
            CriterionResult(criterion_id="c1", marks_awarded=2, feedback="ok"),
            CriterionResult(criterion_id="c2", marks_awarded=3, feedback="ok"),
            CriterionResult(criterion_id="c-bogus", marks_awarded=1, feedback="ok"),
        )
    )
    decision = evaluate(result, RUBRIC)
    assert decision.verdict == GateVerdict.WITHHOLD
    assert any("unknown criteria" in r for r in decision.reasons)


def test_withholds_when_marks_exceed_cap() -> None:
    result = _result(
        criteria=(
            CriterionResult(criterion_id="c1", marks_awarded=99, feedback="ok"),
            CriterionResult(criterion_id="c2", marks_awarded=3, feedback="ok"),
        )
    )
    decision = evaluate(result, RUBRIC)
    assert decision.verdict == GateVerdict.WITHHOLD
    assert any("exceeds max" in r for r in decision.reasons)


def test_retries_on_low_confidence_with_otherwise_valid_result() -> None:
    decision = evaluate(_full_marks_result(confidence=0.2), RUBRIC)
    assert decision.verdict == GateVerdict.RETRY
    assert any("confidence" in r for r in decision.reasons)


def test_structural_violation_never_retries_even_at_high_confidence() -> None:
    result = _result(
        criteria=(CriterionResult(criterion_id="c1", marks_awarded=2, feedback="ok"),),
        confidence=0.99,
    )
    decision = evaluate(result, RUBRIC)
    assert decision.verdict == GateVerdict.WITHHOLD


def test_consistency_accepts_matching_attempts() -> None:
    first = _full_marks_result()
    second = _full_marks_result()
    decision = check_consistency(first, second)
    assert decision.verdict == GateVerdict.ACCEPT


def test_consistency_withholds_on_disagreement_beyond_tolerance() -> None:
    first = _full_marks_result()
    second = _result(
        criteria=(
            CriterionResult(criterion_id="c1", marks_awarded=0, feedback="disagree"),
            CriterionResult(criterion_id="c2", marks_awarded=3, feedback="ok"),
        )
    )
    decision = check_consistency(first, second)
    assert decision.verdict == GateVerdict.WITHHOLD
    assert any("disagree" in r for r in decision.reasons)


def test_consistency_tolerates_small_disagreement() -> None:
    first = _full_marks_result()
    second = _result(
        criteria=(
            CriterionResult(criterion_id="c1", marks_awarded=1, feedback="close"),
            CriterionResult(criterion_id="c2", marks_awarded=3, feedback="ok"),
        )
    )
    decision = check_consistency(first, second, tolerance=1)
    assert decision.verdict == GateVerdict.ACCEPT
