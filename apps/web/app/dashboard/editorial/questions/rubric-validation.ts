import type { CreateQuestionRubricVersionRequest, RubricCriterion } from "./types";

export interface RubricCriterionValues {
  id: string;
  marks: string;
  descriptor: string;
}

export function validateRubricDraft(
  criteria: RubricCriterionValues[],
  maxMarksRaw: string,
): { input: CreateQuestionRubricVersionRequest } | { error: string } {
  if (criteria.length === 0 || criteria.length > 200) {
    return { error: "Add between 1 and 200 criteria." };
  }
  const maxMarks = Number(maxMarksRaw);
  if (!Number.isSafeInteger(maxMarks) || maxMarks <= 0) {
    return { error: "Maximum marks must be a positive integer." };
  }
  const ids = new Set<string>();
  const normalized: RubricCriterion[] = [];
  let total = 0;
  for (const criterion of criteria) {
    const id = criterion.id.trim();
    const marks = Number(criterion.marks);
    if (!id) return { error: "Every criterion needs an id." };
    if (ids.has(id)) return { error: "Criterion ids must be unique." };
    if (!Number.isSafeInteger(marks) || marks <= 0 || marks > 1_000) {
      return { error: "Criterion marks must be positive integers no greater than 1000." };
    }
    if (criterion.descriptor.length > 0 && !criterion.descriptor.trim()) {
      return { error: "Descriptor must contain text or be omitted." };
    }
    ids.add(id);
    total += marks;
    normalized.push({
      id,
      marks,
      ...(criterion.descriptor.trim() ? { descriptor: criterion.descriptor.trim() } : {}),
    });
  }
  if (total !== maxMarks) return { error: "Maximum marks must equal sum of criterion marks." };
  return { input: { rubric: { criteria: normalized }, maxMarks } };
}
