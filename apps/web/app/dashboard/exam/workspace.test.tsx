import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExamWorkspace } from "./workspace";
import type { LearnerQuestion } from "./types";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {}, listExamSyllabuses: vi.fn(), listExamQuestions: vi.fn(), createExamAttempt: vi.fn(), submitExamAttempt: vi.fn(),
}));
import { createExamAttempt, listExamQuestions, listExamSyllabuses, submitExamAttempt } from "./api-client";

const question = (id: string): LearnerQuestion => ({ id, syllabusId: "syl-1", curriculumMapNodeId: "node-1", responseType: "multiple_choice", language: "en", prompt: `opaque-${id}`, options: [{ id: `${id}-a`, label: `opaque-${id}-a` }, { id: `${id}-b`, label: `opaque-${id}-b` }], contentRevision: 1 });
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(listExamSyllabuses).mockResolvedValue({ items: [{ id: "syl-1", board: "Cambridge", syllabusCode: "9700", qualification: "IAL", track: null, displayName: "Biology 9700" }] });
  vi.mocked(listExamQuestions).mockResolvedValue({ items: [question("q1"), question("q2")] });
  vi.mocked(createExamAttempt).mockImplementation(async (id) => ({ attemptId: `attempt-${id}`, questionId: id, status: "open", maxMarks: 1 }));
  vi.mocked(submitExamAttempt).mockImplementation(async (attemptId, selectedOptionId) => ({ attemptId, questionId: attemptId.replace("attempt-", ""), selectedOptionId, correctOptionId: selectedOptionId, isCorrect: true, awardedMarks: 1, maxMarks: 1, feedback: { correctExplanation: "opaque-feedback", incorrectExplanations: [] } }));
});

afterEach(() => {
  vi.useRealTimers();
});

async function start(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByLabelText("Syllabus");
  await user.selectOptions(screen.getByLabelText("Syllabus"), "syl-1");
  await user.click(screen.getByRole("button", { name: /start exam/i }));
}
describe("ExamWorkspace", () => {
  it("requires sufficient MCQs before exam starts", async () => {
    vi.mocked(listExamQuestions).mockResolvedValue({ items: [question("q1")] });
    const user = userEvent.setup(); render(<ExamWorkspace />); await start(user);
    expect(await screen.findByRole("alert")).toHaveTextContent(/need 2 eligible/i);
  });
  it("keeps correctness hidden until confirmed sequential finalization", async () => {
    const user = userEvent.setup(); render(<ExamWorkspace />); await start(user);
    expect(screen.getByText("opaque-q1")).toBeInTheDocument();
    expect(screen.queryByText("opaque-feedback")).not.toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: "opaque-q1-a" }));
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("radio", { name: "opaque-q2-a" }));
    await user.click(screen.getByRole("button", { name: /submit all/i }));
    await user.click(screen.getByRole("button", { name: /confirm submit/i }));
    expect(await screen.findByRole("heading", { name: /exam results/i })).toBeInTheDocument();
    expect(screen.getAllByText("opaque-feedback")).toHaveLength(2);
    expect(createExamAttempt).toHaveBeenCalledTimes(2);
    expect(submitExamAttempt).toHaveBeenCalledTimes(2);
  });
  it("uses one roving radio tab stop and arrow keys select the next option", async () => {
    const user = userEvent.setup(); render(<ExamWorkspace />); await start(user);
    const first = screen.getByRole("radio", { name: "opaque-q1-a" });
    const second = screen.getByRole("radio", { name: "opaque-q1-b" });
    expect(first).toHaveAttribute("tabindex", "0");
    expect(second).toHaveAttribute("tabindex", "-1");
    first.focus(); await user.keyboard("{ArrowRight}");
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute("aria-checked", "true");
  });

  it("keeps timer running during confirmation, submits latest answers at expiry, and labels partial score", async () => {
    vi.useFakeTimers({ now: Date.now() });
    render(<ExamWorkspace durationSeconds={1} />);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    fireEvent.change(screen.getByLabelText("Syllabus"), { target: { value: "syl-1" } });
    fireEvent.submit(screen.getByRole("button", { name: /start exam/i }).closest("form")!);
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    fireEvent.click(screen.getByRole("radio", { name: "opaque-q1-a" }));
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.click(screen.getByRole("radio", { name: "opaque-q2-a" }));
    fireEvent.click(screen.getByRole("button", { name: "Submit all" }));
    try {
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Confirm submit" })).toHaveFocus();
      await act(async () => {
        vi.advanceTimersByTime(1000);
        await Promise.resolve();
      });
      expect(screen.getByRole("heading", { name: "Exam results" })).toBeInTheDocument();
      expect(createExamAttempt).toHaveBeenCalledTimes(2);
      expect(submitExamAttempt).toHaveBeenCalledWith("attempt-q1", "q1-a");
      expect(submitExamAttempt).toHaveBeenCalledWith("attempt-q2", "q2-a");
      expect(screen.getByText(/answered score: 2 \/ 2/i)).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("traps confirmation focus and returns it to submit trigger when continuing", async () => {
    const user = userEvent.setup();
    render(<ExamWorkspace />);
    await start(user);
    await user.click(screen.getByRole("button", { name: "Submit all" }));
    const confirm = screen.getByRole("button", { name: "Confirm submit" });
    const continueExam = screen.getByRole("button", { name: "Continue exam" });
    expect(confirm).toHaveFocus();
    continueExam.focus();
    await user.keyboard("{Tab}");
    expect(confirm).toHaveFocus();
    await user.click(continueExam);
    expect(screen.getByRole("button", { name: "Submit all" })).toHaveFocus();
  });
});
