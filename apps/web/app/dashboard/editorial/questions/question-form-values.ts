import type { CreateQuestionRequest, Question, QuestionResponseType, UpdateQuestionRequest } from "./types";

export interface QuestionFieldValues {
  curriculumMapNodeId: string;
  responseType: "" | QuestionResponseType;
  language: string;
  prompt: string;
}

export const EMPTY_QUESTION_VALUES: QuestionFieldValues = {
  curriculumMapNodeId: "",
  responseType: "",
  language: "",
  prompt: "",
};

export function valuesFromQuestion(question: Question): QuestionFieldValues {
  return {
    curriculumMapNodeId: question.curriculumMapNodeId,
    responseType: question.responseType,
    language: question.language,
    prompt: question.prompt,
  };
}

function validate(values: QuestionFieldValues): string | null {
  if (!values.curriculumMapNodeId || !values.responseType || !values.language.trim() || !values.prompt.trim()) {
    return "Node, response type, language, and prompt are required.";
  }
  return null;
}

export function buildCreateInput(
  syllabusId: string,
  values: QuestionFieldValues,
): { input: CreateQuestionRequest } | { error: string } {
  const error = validate(values);
  if (error) return { error };
  return {
    input: {
      syllabusId,
      curriculumMapNodeId: values.curriculumMapNodeId,
      responseType: values.responseType as QuestionResponseType,
      language: values.language.trim(),
      prompt: values.prompt.trim(),
    },
  };
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
  if (Object.keys(patch).length === 0) return { error: "Change at least one field before saving." };
  return { patch };
}
