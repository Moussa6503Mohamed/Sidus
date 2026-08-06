import { describe, expect, it } from "vitest";
import { validateRubricDraft } from "./rubric-validation";

describe("validateRubricDraft", () => {
  const shortAnswer = { responseType: "short_answer" as const, options: null };
  const multipleChoice = { responseType: "multiple_choice" as const, options: [{ id: "o1", label: "runtime label" }, { id: "o2", label: "runtime label 2" }] };

  it("rejects empty criteria", () => {
    expect(validateRubricDraft([], "1", shortAnswer, "")).toEqual({ error: "Add between 1 and 200 criteria." });
  });

  it("rejects duplicate ids and non-integer marks", () => {
    const id = crypto.randomUUID();
    expect(validateRubricDraft([
      { id, marks: "1", descriptor: "" },
      { id, marks: "1", descriptor: "" },
    ], "2", shortAnswer, "")).toEqual({ error: "Criterion ids must be unique." });
    expect(validateRubricDraft([{ id, marks: "1.5", descriptor: "" }], "1", shortAnswer, "")).toEqual({
      error: "Criterion marks must be positive integers no greater than 1000.",
    });
  });

  it("requires maximum marks to equal criterion sum and emits exact safe shape", () => {
    const id = crypto.randomUUID();
    expect(validateRubricDraft([{ id, marks: "2", descriptor: "" }], "1", shortAnswer, "")).toEqual({
      error: "Maximum marks must equal sum of criterion marks.",
    });
    expect(validateRubricDraft([{ id, marks: "2", descriptor: "" }], "2", shortAnswer, "")).toEqual({
      input: { rubric: { criteria: [{ id, marks: 2 }] }, maxMarks: 2 },
    });
  });

  it("requires MCQ answer key from current options", () => {
    const criteria = [{ id: "c1", marks: "1", descriptor: "" }];
    expect(validateRubricDraft(criteria, "1", multipleChoice, "missing")).toEqual({
      error: "Select a correct option from current question options.",
    });
    expect(validateRubricDraft(criteria, "1", multipleChoice, "o2", "opaque-c", { o1: "opaque-w" })).toEqual({
      input: { rubric: { criteria: [{ id: "c1", marks: 1 }], answerKey: { correctOptionId: "o2" }, feedback: { correctExplanation: "opaque-c", incorrectExplanations: [{ optionId: "o1", explanation: "opaque-w" }] } }, maxMarks: 1 },
    });
  });
});
