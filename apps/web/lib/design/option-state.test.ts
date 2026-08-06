import { describe, expect, it } from "vitest";
import { getOptionState } from "./option-state";

describe("getOptionState", () => {
  it("returns default with no tag before marking, unselected, not disabled", () => {
    expect(getOptionState({ optionId: "a", isMarked: false })).toEqual({ state: "default", tag: null });
  });

  it("returns selected with a Selected tag before marking when chosen", () => {
    expect(getOptionState({ optionId: "a", selectedOptionId: "a", isMarked: false })).toEqual({
      state: "selected",
      tag: "Selected",
    });
  });

  it("returns disabled with a Locked tag while submitting, even if selected", () => {
    expect(getOptionState({ optionId: "a", selectedOptionId: "a", isMarked: false, disabled: true })).toEqual({
      state: "disabled",
      tag: "Locked",
    });
  });

  it("returns correct with 'Correct answer' when marked, correct, and not the learner's pick", () => {
    expect(getOptionState({ optionId: "a", selectedOptionId: "b", correctOptionId: "a", isMarked: true })).toEqual({
      state: "correct",
      tag: "Correct answer",
    });
  });

  it("returns correct with 'Your answer · correct' when marked, correct, and selected", () => {
    expect(getOptionState({ optionId: "a", selectedOptionId: "a", correctOptionId: "a", isMarked: true })).toEqual({
      state: "correct",
      tag: "Your answer · correct",
    });
  });

  it("returns selected-incorrect when marked, selected, and wrong", () => {
    expect(getOptionState({ optionId: "b", selectedOptionId: "b", correctOptionId: "a", isMarked: true })).toEqual({
      state: "selected-incorrect",
      tag: "Your answer · incorrect",
    });
  });

  it("returns incorrect when marked, neither selected nor correct", () => {
    expect(getOptionState({ optionId: "c", selectedOptionId: "b", correctOptionId: "a", isMarked: true })).toEqual({
      state: "incorrect",
      tag: "Not correct",
    });
  });

  it("every marked state carries a non-null text tag — correctness is never colour-only", () => {
    const states = [
      getOptionState({ optionId: "a", selectedOptionId: "a", correctOptionId: "a", isMarked: true }),
      getOptionState({ optionId: "b", selectedOptionId: "b", correctOptionId: "a", isMarked: true }),
      getOptionState({ optionId: "c", selectedOptionId: "b", correctOptionId: "a", isMarked: true }),
    ];
    for (const s of states) expect(s.tag).not.toBeNull();
  });
});
