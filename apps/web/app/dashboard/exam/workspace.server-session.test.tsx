import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ExamWorkspace } from "./workspace";
import type { AssessmentSession, AssessmentSessionResult } from "./api-client";
import type { LearnerQuestion } from "./types";

// Companion to workspace.test.tsx, which stubs createAssessmentSession/saveAssessmentResponse/
// submitAssessmentSession as undefined to exercise the legacy in-memory fallback path. This file
// mocks them as real functions instead, so ExamWorkspace takes the server-owned session branch
// (workspace.tsx's `typeof createAssessmentSession !== "function"` guard) and never falls back.
vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  listExamSyllabuses: vi.fn(),
  listExamModules: vi.fn(),
  listExamQuestions: vi.fn(),
  createExamAttempt: vi.fn(),
  submitExamAttempt: vi.fn(),
  createAssessmentSession: vi.fn(),
  saveAssessmentResponse: vi.fn(),
  submitAssessmentSession: vi.fn(),
}));
import {
  createAssessmentSession,
  createExamAttempt,
  listExamModules,
  listExamSyllabuses,
  saveAssessmentResponse,
  submitAssessmentSession,
  submitExamAttempt,
} from "./api-client";

const question = (id: string): LearnerQuestion => ({
  id,
  syllabusId: "syl-1",
  curriculumMapNodeId: "node-1",
  responseType: "multiple_choice",
  language: "en",
  prompt: `opaque-${id}`,
  options: [{ id: `${id}-a`, label: `opaque-${id}-a` }, { id: `${id}-b`, label: `opaque-${id}-b` }],
  contentRevision: 1,
});

function session(): AssessmentSession {
  return {
    id: "sess-1",
    mode: "exam",
    syllabusId: "syl-1",
    curriculumMapNodeId: "module-1",
    status: "open",
    startedAt: new Date().toISOString(),
    deadlineAt: new Date(Date.now() + 30 * 60 * 1000).toISOString(),
    version: 0,
    items: [
      { ordinal: 1, question: question("q1"), responseVersion: 0 },
      { ordinal: 2, question: question("q2"), responseVersion: 0 },
    ],
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(listExamSyllabuses).mockResolvedValue({ items: [{ id: "syl-1", board: "Cambridge", syllabusCode: "9700", qualification: "IAL", track: null, displayName: "Biology 9700" }] });
  vi.mocked(listExamModules).mockResolvedValue({ items: [{ id: "module-1", syllabusId: "syl-1", code: "M1", label: "opaque-module" }] });
  vi.mocked(createAssessmentSession).mockResolvedValue(session());
  vi.mocked(saveAssessmentResponse).mockImplementation(async (_id, body) => ({ ordinal: body.ordinal, question: question(body.ordinal === 1 ? "q1" : "q2"), answer: body.answer, responseVersion: body.expectedVersion + 1 }));
});

async function start(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByLabelText("Syllabus");
  await user.selectOptions(screen.getByLabelText("Syllabus"), "syl-1");
  await screen.findByLabelText("Module");
  await user.selectOptions(screen.getByLabelText("Module"), "module-1");
  await user.click(screen.getByRole("button", { name: /start exam/i }));
}

describe("ExamWorkspace server-owned session", () => {
  it("creates a server session on start instead of the legacy in-memory path", async () => {
    const user = userEvent.setup();
    render(<ExamWorkspace />);
    await start(user);
    expect(await screen.findByText("opaque-q1")).toBeInTheDocument();
    expect(createAssessmentSession).toHaveBeenCalledWith({ mode: "exam", syllabusId: "syl-1", curriculumMapNodeId: "module-1", questionCount: 500, durationSeconds: 30 * 60 });
    expect(listExamModules).toHaveBeenCalled();
    // Legacy per-question attempt creation must never run once a server session exists.
    expect(createExamAttempt).not.toHaveBeenCalled();
  });

  it("autosaves each answer against the server session with the pinned expected version", async () => {
    const user = userEvent.setup();
    render(<ExamWorkspace />);
    await start(user);
    await user.click(screen.getByRole("radio", { name: "opaque-q1-a" }));
    expect(saveAssessmentResponse).toHaveBeenCalledWith("sess-1", { ordinal: 1, expectedVersion: 0, answer: { selectedOptionId: "q1-a" } });
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("radio", { name: "opaque-q2-a" }));
    expect(saveAssessmentResponse).toHaveBeenCalledWith("sess-1", { ordinal: 2, expectedVersion: 0, answer: { selectedOptionId: "q2-a" } });
    expect(saveAssessmentResponse).toHaveBeenCalledTimes(2);
  });

  it("submits the server session (not per-attempt submission) and renders its returned results", async () => {
    const result: AssessmentSessionResult = {
      ...session(),
      status: "submitted",
      results: [
        { attemptId: "attempt-q1", questionId: "q1", selectedOptionId: "q1-a", correctOptionId: "q1-a", isCorrect: true, awardedMarks: 1, maxMarks: 1, feedback: { correctExplanation: "opaque-feedback-1", incorrectExplanations: [] } },
        { attemptId: "attempt-q2", questionId: "q2", selectedOptionId: "q2-b", correctOptionId: "q2-a", isCorrect: false, awardedMarks: 0, maxMarks: 1, feedback: { correctExplanation: "opaque-feedback-2", incorrectExplanations: [] } },
      ],
    };
    vi.mocked(submitAssessmentSession).mockResolvedValue(result);
    const user = userEvent.setup();
    render(<ExamWorkspace />);
    await start(user);
    await user.click(screen.getByRole("radio", { name: "opaque-q1-a" }));
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("radio", { name: "opaque-q2-a" }));
    await user.click(screen.getByRole("button", { name: /submit all/i }));
    await user.click(screen.getByRole("button", { name: /confirm submit/i }));
    expect(await screen.findByRole("heading", { name: /exam results/i })).toBeInTheDocument();
    expect(submitAssessmentSession).toHaveBeenCalledWith("sess-1");
    // The finalized score/labels must come from the session result, never from a per-attempt submit call.
    expect(submitExamAttempt).not.toHaveBeenCalled();
    expect(screen.getByText(/answered score: 1 \/ 2/i)).toBeInTheDocument();
    expect(screen.getByText("opaque-feedback-1")).toBeInTheDocument();
    expect(screen.getByText("opaque-feedback-2")).toBeInTheDocument();
  });

  it("surfaces a retry on session submit failure without falling back to legacy finalization", async () => {
    vi.mocked(submitAssessmentSession).mockRejectedValueOnce(new Error("offline"));
    const user = userEvent.setup();
    render(<ExamWorkspace />);
    await start(user);
    await user.click(screen.getByRole("radio", { name: "opaque-q1-a" }));
    await user.click(screen.getByRole("button", { name: /submit all/i }));
    await user.click(screen.getByRole("button", { name: /confirm submit/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/retry without changing answers/i);
    expect(createExamAttempt).not.toHaveBeenCalled();
    expect(submitExamAttempt).not.toHaveBeenCalled();
  });
});
