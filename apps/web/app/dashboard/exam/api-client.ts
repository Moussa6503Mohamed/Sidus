export {
  ApiError,
  createPracticeAttempt as createExamAttempt,
  listPracticeQuestions as listExamQuestions,
  listPracticeModules as listExamModules,
  listPracticeSyllabuses as listExamSyllabuses,
} from "../practice/api-client";
import { ApiError, submitPracticeAttempt } from "../practice/api-client";
import type { LearnerAnswer, LearnerAttemptResult } from "./types";

export interface AssessmentSessionItem { ordinal: number; question: import("./types").LearnerQuestion; answer?: LearnerAnswer; responseVersion: number; }
export interface AssessmentSession { id: string; mode: "practice" | "exam"; syllabusId: string; curriculumMapNodeId?: string; status: "open" | "submitted" | "expired"; startedAt: string; deadlineAt?: string; version: number; items: AssessmentSessionItem[]; }
export interface AssessmentSessionResult extends AssessmentSession { results: LearnerAttemptResult[]; }
async function sessionRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { ...init, headers: { Accept: "application/json", ...init?.headers } });
  const text = await res.text(); const body: unknown = text ? JSON.parse(text) : null;
  if (!res.ok) { const b = body as { error?: string; message?: string } | null; throw new ApiError(res.status, b?.error ?? "request_failed", b?.message ?? "The assessment service could not be completed."); }
  return body as T;
}
export function createAssessmentSession(input: { mode: "practice" | "exam"; syllabusId: string; curriculumMapNodeId?: string; questionCount: number; durationSeconds?: number }): Promise<AssessmentSession> { return sessionRequest("/api/learner/assessment-sessions", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) }); }
export function getAssessmentSession(id: string): Promise<AssessmentSession> { return sessionRequest(`/api/learner/assessment-sessions/${encodeURIComponent(id)}`); }
export function saveAssessmentResponse(id: string, body: { ordinal: number; expectedVersion: number; answer: LearnerAnswer }): Promise<AssessmentSessionItem> { return sessionRequest(`/api/learner/assessment-sessions/${encodeURIComponent(id)}/responses`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); }
export function submitAssessmentSession(id: string): Promise<AssessmentSessionResult> { return sessionRequest(`/api/learner/assessment-sessions/${encodeURIComponent(id)}/submit`, { method: "POST" }); }
export function getAssessmentResult(id: string): Promise<AssessmentSessionResult> { return sessionRequest(`/api/learner/assessment-sessions/${encodeURIComponent(id)}/result`); }

export function submitExamAttempt(attemptId: string, answer: any): Promise<LearnerAttemptResult> {
	return submitPracticeAttempt(attemptId, typeof answer === "string" ? { selectedOptionId: answer } : answer);
}
