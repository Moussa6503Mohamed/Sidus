import { describe, expect, it } from "vitest";
import { validateRubricDraft } from "./rubric-validation";

describe("validateRubricDraft", () => {
  it("rejects empty criteria", () => {
    expect(validateRubricDraft([], "1")).toEqual({ error: "Add between 1 and 200 criteria." });
  });

  it("rejects duplicate ids and non-integer marks", () => {
    const id = crypto.randomUUID();
    expect(validateRubricDraft([
      { id, marks: "1", descriptor: "" },
      { id, marks: "1", descriptor: "" },
    ], "2")).toEqual({ error: "Criterion ids must be unique." });
    expect(validateRubricDraft([{ id, marks: "1.5", descriptor: "" }], "1")).toEqual({
      error: "Criterion marks must be positive integers no greater than 1000.",
    });
  });

  it("requires maximum marks to equal criterion sum and emits exact safe shape", () => {
    const id = crypto.randomUUID();
    expect(validateRubricDraft([{ id, marks: "2", descriptor: "" }], "1")).toEqual({
      error: "Maximum marks must equal sum of criterion marks.",
    });
    expect(validateRubricDraft([{ id, marks: "2", descriptor: "" }], "2")).toEqual({
      input: { rubric: { criteria: [{ id, marks: 2 }] }, maxMarks: 2 },
    });
  });
});
