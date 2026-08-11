import type { LearnerAnswer, LearnerAttempt, LearnerAttemptResult, LearnerModule, LearnerQuestion, LearnerSyllabus } from "./types";

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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { ...init, headers: { Accept: "application/json", ...init?.headers } });
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

export function listPracticeModules(syllabusId: string): Promise<ListResponse<LearnerModule>> {
  return request(`/api/learner/modules?${new URLSearchParams({ syllabusId }).toString()}`);
}

export function createPracticeAttempt(questionId: string): Promise<LearnerAttempt> {
  return request(`/api/learner/questions/${encodeURIComponent(questionId)}/attempts`, { method: "POST" });
}

export function submitPracticeAttempt(attemptId: string, answer: LearnerAnswer): Promise<LearnerAttemptResult> {
  return request(`/api/learner/attempts/${encodeURIComponent(attemptId)}/submit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(answer),
  });
}
