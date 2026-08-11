export {
  ApiError,
  createPracticeAttempt as createExamAttempt,
  listPracticeQuestions as listExamQuestions,
  listPracticeModules as listExamModules,
  listPracticeSyllabuses as listExamSyllabuses,
} from "../practice/api-client";
import { submitPracticeAttempt } from "../practice/api-client";
import type { LearnerAnswer, LearnerAttemptResult } from "./types";

export function submitExamAttempt(attemptId: string, answer: any): Promise<LearnerAttemptResult> {
	return submitPracticeAttempt(attemptId, typeof answer === "string" ? { selectedOptionId: answer } : answer);
}
