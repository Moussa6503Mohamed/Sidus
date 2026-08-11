import type { LearnerAnswer, LearnerAttemptResult, LearnerQuestion } from "./types";

export interface ExamRuntime {
  attempts: Record<string, string>;
  results: Record<string, LearnerAttemptResult>;
}

export interface ExamOperations {
  createAttempt(questionId: string): Promise<{ attemptId: string }>;
  submitAttempt(attemptId: string, answer: LearnerAnswer): Promise<LearnerAttemptResult>;
}

/** Sequential, idempotent for current page lifetime. Caller retains runtime for retry. */
export async function finalizeExam(
  questions: LearnerQuestion[],
  answers: Record<string, LearnerAnswer | undefined>,
  runtime: ExamRuntime,
  operations: ExamOperations,
  onProgress?: (completed: number, total: number) => void,
): Promise<void> {
  const answered = questions.filter((question) => Boolean(answers[question.id]));
  let completed = Object.keys(runtime.results).length;
  onProgress?.(completed, answered.length);
  for (const question of answered) {
    if (runtime.results[question.id]) continue;
    let attemptId = runtime.attempts[question.id];
    if (!attemptId) {
      attemptId = (await operations.createAttempt(question.id)).attemptId;
      runtime.attempts[question.id] = attemptId;
    }
    runtime.results[question.id] = await operations.submitAttempt(attemptId, answers[question.id]!);
    completed += 1;
    onProgress?.(completed, answered.length);
  }
}
