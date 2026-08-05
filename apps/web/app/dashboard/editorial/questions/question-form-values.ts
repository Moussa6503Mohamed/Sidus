import type { CreateQuestionRequest, MultipleChoiceOption, Question, QuestionResponseType, UpdateQuestionRequest } from "./types";

export interface QuestionFieldValues {
  curriculumMapNodeId: string;
  responseType: "" | QuestionResponseType;
  language: string;
  prompt: string;
  options: MultipleChoiceOption[];
}

export const EMPTY_QUESTION_VALUES: QuestionFieldValues = {
  curriculumMapNodeId: "",
  responseType: "",
  language: "",
  prompt: "",
  options: [],
};

export function valuesFromQuestion(question: Question): QuestionFieldValues {
  return {
    curriculumMapNodeId: question.curriculumMapNodeId,
    responseType: question.responseType,
    language: question.language,
    prompt: question.prompt,
    options: question.options ?? [],
  };
}

function validate(values: QuestionFieldValues): string | null {
  if (!values.curriculumMapNodeId || !values.responseType || !values.language.trim() || !values.prompt.trim()) {
    return "Node, response type, language, and prompt are required.";
  }
  if (values.responseType === "multiple_choice") {
    if (values.options.length < 2 || values.options.length > 6) return "Add between 2 and 6 options.";
    const ids = new Set<string>();
    for (const option of values.options) {
      const id = option.id.trim();
      const label = option.label.trim();
      if (!id || !label) return "Every option needs an id and label.";
      if ([...id].length > 64 || [...label].length > 1000) return "Option ids must be at most 64 characters and labels at most 1000.";
      if (ids.has(id)) return "Option ids must be unique.";
      ids.add(id);
    }
  }
  return null;
}

function normalizedOptions(options: MultipleChoiceOption[]): MultipleChoiceOption[] {
  return options.map((option) => ({ id: option.id.trim(), label: option.label.trim() }));
}

export function buildCreateInput(
  syllabusId: string,
  values: QuestionFieldValues,
): { input: CreateQuestionRequest } | { error: string } {
  const error = validate(values);
  if (error) return { error };
  const common = {
    syllabusId,
    curriculumMapNodeId: values.curriculumMapNodeId,
    language: values.language.trim(),
    prompt: values.prompt.trim(),
  };
  if (values.responseType === "multiple_choice") {
    return { input: { ...common, responseType: values.responseType, options: normalizedOptions(values.options) } };
  }
  return { input: { ...common, responseType: values.responseType as "short_answer" | "structured_response" } };
}

export function buildUpdatePatch(
  original: QuestionFieldValues,
  values: QuestionFieldValues,
): { patch: UpdateQuestionRequest } | { error: string } {
  const error = validate(values);
  if (error) return { error };
  const patch: UpdateQuestionRequest = {};
  if (values.curriculumMapNodeId !== original.curriculumMapNodeId) patch.curriculumMapNodeId = values.curriculumMapNodeId;
  if (values.responseType !== original.responseType) patch.responseType = values.responseType as QuestionResponseType;
  if (values.language.trim() !== original.language) patch.language = values.language.trim();
  if (values.prompt.trim() !== original.prompt) patch.prompt = values.prompt.trim();
  if (values.responseType === "multiple_choice") {
    const next = normalizedOptions(values.options);
    if (JSON.stringify(next) !== JSON.stringify(original.options)) patch.options = next;
  }
  if (Object.keys(patch).length === 0) return { error: "Change at least one field before saving." };
  return { patch };
}
