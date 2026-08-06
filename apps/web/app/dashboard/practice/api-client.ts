import type { LearnerQuestion, LearnerSyllabus } from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

interface ListResponse<T> {
  items: T[];
}

async function request<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  const text = await response.text();
  const body: unknown = text ? JSON.parse(text) : null;

  if (!response.ok) {
    const errorBody = body as { error?: unknown; message?: unknown } | null;
    const code = typeof errorBody?.error === "string" ? errorBody.error : "request_failed";
    const message =
      typeof errorBody?.message === "string" ? errorBody.message : "The request could not be completed.";
    throw new ApiError(response.status, code, message);
  }

  return body as T;
}

export function listPracticeQuestions(
  syllabusId: string,
  curriculumMapNodeId?: string,
): Promise<ListResponse<LearnerQuestion>> {
  const query = new URLSearchParams({ syllabusId });
  if (curriculumMapNodeId) query.set("curriculumMapNodeId", curriculumMapNodeId);
  return request(`/api/learner/questions?${query.toString()}`);
}

export function listPracticeSyllabuses(): Promise<ListResponse<LearnerSyllabus>> {
  return request(`/api/learner/syllabuses`);
}
