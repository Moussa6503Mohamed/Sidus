import { describe, expect, it, vi } from "vitest";
import { finalizeExam } from "./finalization";

const questions = [
  { id: "q-1", options: [{ id: "a", label: "opaque-a" }] },
  { id: "q-2", options: [{ id: "b", label: "opaque-b" }] },
] as never[];

describe("finalizeExam", () => {
  it("creates then submits answered questions sequentially and skips unanswered", async () => {
    const createAttempt = vi.fn().mockResolvedValueOnce({ attemptId: "attempt-1" });
    const submitAttempt = vi.fn().mockResolvedValueOnce({ attemptId: "attempt-1", questionId: "q-1", selectedOptionId: "a", correctOptionId: "a", isCorrect: true, awardedMarks: 1, maxMarks: 1, feedback: { correctExplanation: "opaque", incorrectExplanations: [] } });
    const runtime = { attempts: {}, results: {} };
    await finalizeExam(questions, { "q-1": "a" }, runtime, { createAttempt, submitAttempt });
    expect(createAttempt).toHaveBeenCalledTimes(1);
    expect(createAttempt).toHaveBeenCalledWith("q-1");
    expect(submitAttempt).toHaveBeenCalledWith("attempt-1", "a");
    expect(runtime.results).toHaveProperty("q-1");
  });

  it("retries only incomplete submission with retained attempt id", async () => {
    const createAttempt = vi.fn().mockResolvedValue({ attemptId: "attempt-1" });
    const submitAttempt = vi.fn().mockRejectedValueOnce(new Error("temporary")).mockResolvedValueOnce({ attemptId: "attempt-1", questionId: "q-1", selectedOptionId: "a", correctOptionId: "a", isCorrect: true, awardedMarks: 1, maxMarks: 1, feedback: { correctExplanation: "opaque", incorrectExplanations: [] } });
    const runtime = { attempts: {}, results: {} };
    await expect(finalizeExam(questions, { "q-1": "a" }, runtime, { createAttempt, submitAttempt })).rejects.toThrow("temporary");
    await finalizeExam(questions, { "q-1": "a" }, runtime, { createAttempt, submitAttempt });
    expect(createAttempt).toHaveBeenCalledTimes(1);
    expect(submitAttempt).toHaveBeenCalledTimes(2);
  });
});
