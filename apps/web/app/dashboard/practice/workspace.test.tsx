import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PracticeWorkspace } from "./workspace";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {
    status: number;
    code: string;
    constructor(status: number, code: string, message: string) {
      super(message);
      this.status = status;
      this.code = code;
    }
  },
  listPracticeQuestions: vi.fn(),
  listPracticeSyllabuses: vi.fn(),
  listPracticeModules: vi.fn(),
  createPracticeAttempt: vi.fn(),
  submitPracticeAttempt: vi.fn(),
}));

import { ApiError, createPracticeAttempt, listPracticeModules, listPracticeQuestions, listPracticeSyllabuses, submitPracticeAttempt } from "./api-client";

const mockedListQuestions = vi.mocked(listPracticeQuestions);
const mockedListSyllabuses = vi.mocked(listPracticeSyllabuses);
const mockedListModules = vi.mocked(listPracticeModules);
const mockedCreateAttempt = vi.mocked(createPracticeAttempt);
const mockedSubmitAttempt = vi.mocked(submitPracticeAttempt);

const oneActiveSyllabus = {
  items: [
    { id: "syl-1", board: "Cambridge", syllabusCode: "0610", qualification: "IGCSE", track: "Extended", displayName: "Biology 0610" },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  mockedListSyllabuses.mockResolvedValue(oneActiveSyllabus);
  mockedListModules.mockResolvedValue({ items: [{ id: "module-1", syllabusId: "syl-1", code: "M1", label: "opaque-module" }] });
  mockedCreateAttempt.mockResolvedValue({ attemptId: "attempt-1", questionId: "q-1", status: "open", maxMarks: 2 });
  mockedSubmitAttempt.mockResolvedValue({
    attemptId: "attempt-1", questionId: "q-1", selectedOptionId: "opt-b", correctOptionId: "opt-a",
    isCorrect: false, awardedMarks: 0, maxMarks: 2,
    feedback: { correctExplanation: "opaque-correct", incorrectExplanations: [{ optionId: "opt-b", explanation: "opaque-wrong" }] },
  });
});

async function submit(user: ReturnType<typeof userEvent.setup>, syllabusId = "syl-1") {
  await screen.findByRole("combobox", { name: /syllabus/i });
  await user.selectOptions(screen.getByRole("combobox", { name: /syllabus/i }), syllabusId);
  await user.click(screen.getByRole("button", { name: /load questions/i }));
}

describe("PracticeWorkspace — syllabus discovery", () => {
  it("fetches active syllabuses on mount and shows a loading state first", () => {
    mockedListSyllabuses.mockReturnValue(new Promise(() => {})); // never resolves
    render(<PracticeWorkspace />);

    expect(mockedListSyllabuses).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/loading syllabuses/i)).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /syllabus/i })).not.toBeInTheDocument();
  });

  it("never asks the learner to type a raw syllabus id — the syllabus field is a select, not a text box", async () => {
    render(<PracticeWorkspace />);

    const select = await screen.findByRole("combobox", { name: /syllabus/i });
    expect(select.tagName).toBe("SELECT");
    expect(screen.getByRole("option", { name: /biology 0610 \(extended\)/i })).toBeInTheDocument();
  });

  it("shows an empty state when no active syllabuses exist, and renders no picker", async () => {
    mockedListSyllabuses.mockResolvedValue({ items: [] });
    render(<PracticeWorkspace />);

    expect(await screen.findByText(/no active syllabuses are available/i)).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /syllabus/i })).not.toBeInTheDocument();
    expect(mockedListQuestions).not.toHaveBeenCalled();
  });

  it("shows an error with retry when syllabus discovery fails, and retry calls it again", async () => {
    mockedListSyllabuses.mockRejectedValueOnce(new ApiError(502, "upstream_error", "practice service unavailable"));
    mockedListSyllabuses.mockResolvedValueOnce(oneActiveSyllabus);
    const user = userEvent.setup();
    render(<PracticeWorkspace />);

    expect(await screen.findByRole("alert")).toHaveTextContent(/practice service unavailable/i);
    expect(screen.queryByRole("combobox", { name: /syllabus/i })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /retry/i }));

    await screen.findByRole("combobox", { name: /syllabus/i });
    expect(mockedListSyllabuses).toHaveBeenCalledTimes(2);
  });

  it("does not fetch questions until the learner selects a syllabus and submits", async () => {
    render(<PracticeWorkspace />);
    await screen.findByRole("combobox", { name: /syllabus/i });

    expect(mockedListQuestions).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /load questions/i })).toBeDisabled();
  });
});

describe("PracticeWorkspace — question loading", () => {
  it("loads learner-safe modules and renders eligible questions for selected module", async () => {
    mockedListQuestions.mockResolvedValue({
      items: [
        {
          id: "q-1",
          syllabusId: "syl-1",
          curriculumMapNodeId: "node-1",
          responseType: "multiple_choice",
          language: "en",
          prompt: "opaque-prompt-1",
          options: [
            { id: "opt-a", label: "opaque-a" },
            { id: "opt-b", label: "opaque-b" },
          ],
          contentRevision: 1,
        },
      ],
    });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);

    await screen.findByRole("combobox", { name: /syllabus/i });
    await user.selectOptions(screen.getByRole("combobox", { name: /syllabus/i }), "syl-1");
    await screen.findByRole("combobox", { name: /module/i });
    await user.selectOptions(screen.getByRole("combobox", { name: /module/i }), "module-1");
    await user.click(screen.getByRole("button", { name: /load questions/i }));

    expect(await screen.findByText("opaque-prompt-1")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "opaque-a" })).toBeInTheDocument();
    expect(mockedListModules).toHaveBeenCalledWith("syl-1");
    expect(mockedListQuestions).toHaveBeenCalledWith("syl-1", "module-1");
  });

  it("never asks learner to type raw node identifiers", async () => {
    render(<PracticeWorkspace />);
    const user = userEvent.setup();
    await screen.findByLabelText(/syllabus/i);
    await user.selectOptions(screen.getByLabelText(/syllabus/i), "syl-1");
    expect(await screen.findByLabelText(/^module$/i)).toBeInstanceOf(HTMLSelectElement);
    expect(screen.queryByText(/node/i)).not.toBeInTheDocument();
  });

  it("uses All available by default and rejects count above availability without truncating", async () => {
    mockedListQuestions.mockResolvedValue({ items: [{
      id: "q-1", syllabusId: "syl-1", curriculumMapNodeId: "module-1", responseType: "multiple_choice", language: "en", prompt: "opaque-one", options: [{ id: "a", label: "a" }, { id: "b", label: "b" }], contentRevision: 1,
    }] });
    const user = userEvent.setup(); render(<PracticeWorkspace />);
    await screen.findByLabelText(/syllabus/i);
    await user.selectOptions(screen.getByLabelText(/syllabus/i), "syl-1");
    await user.click(screen.getByRole("button", { name: /load questions/i }));
    expect(await screen.findByText(/showing 1 of 1/i)).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText(/^questions$/i), "custom");
    await user.clear(screen.getByLabelText(/question count/i));
    await user.type(screen.getByLabelText(/question count/i), "2");
    await user.click(screen.getByRole("button", { name: /load questions/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/1 to 1/i);
  });

  it("shows an empty state when no questions are eligible", async () => {
    mockedListQuestions.mockResolvedValue({ items: [] });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);

    await submit(user);

    expect(await screen.findByText(/no eligible questions yet/i)).toBeInTheDocument();
  });

  it("shows an error with retry, and retry calls the API again", async () => {
    mockedListQuestions.mockRejectedValueOnce(new ApiError(400, "unknown_syllabus", "syllabus is unknown or not active"));
    mockedListQuestions.mockResolvedValueOnce({ items: [] });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);

    await submit(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(/syllabus is unknown or not active/i);

    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => expect(mockedListQuestions).toHaveBeenCalledTimes(2));
  });

  it("submits once and renders score, selected/correct labels, and complete verified feedback", async () => {
    mockedListQuestions.mockResolvedValue({
      items: [
        {
          id: "q-1",
          syllabusId: "syl-1",
          curriculumMapNodeId: "node-1",
          responseType: "multiple_choice",
          language: "en",
          prompt: "opaque-prompt-1",
          options: [{ id: "opt-a", label: "opaque-a" }, { id: "opt-b", label: "opaque-b" }],
          contentRevision: 1,
        },
      ],
    });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);
    await submit(user);
    await screen.findByText("opaque-prompt-1");

    const optionButton = screen.getByRole("radio", { name: "opaque-b" });
    expect(optionButton).toHaveAttribute("aria-checked", "false");

    await user.click(optionButton);

    expect(optionButton).toHaveAttribute("aria-checked", "true");
    expect(mockedCreateAttempt).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /submit answer/i }));
    await screen.findByRole("region", { name: /answer feedback/i });
    expect(mockedCreateAttempt).toHaveBeenCalledWith("q-1");
    expect(mockedSubmitAttempt).toHaveBeenCalledWith("attempt-1", { selectedOptionId: "opt-b" });
    expect(screen.getByText(/0 \/ 2/)).toBeInTheDocument();
    expect(screen.getByText("opaque-correct")).toBeInTheDocument();
    expect(screen.getByText("opaque-wrong")).toBeInTheDocument();
    // Non-color-only MCQ result states: each marked option carries an explicit text tag in
    // addition to its border/colour treatment (lib/design/option-state.ts).
    expect(screen.getByText("Correct answer")).toBeInTheDocument();
    expect(screen.getByText("Your answer · incorrect")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /submit answer/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /start/i })).not.toBeInTheDocument();
  });

  it("keeps pinned attempt for retry after submit failure and prevents duplicate create", async () => {
    mockedListQuestions.mockResolvedValue({ items: [{
      id: "q-1", syllabusId: "syl-1", curriculumMapNodeId: "node-1", responseType: "multiple_choice",
      language: "en", prompt: "opaque-prompt", options: [{ id: "opt-a", label: "opaque-a" }, { id: "opt-b", label: "opaque-b" }], contentRevision: 1,
    }] });
    mockedSubmitAttempt.mockRejectedValueOnce(new ApiError(502, "upstream", "opaque-failure"));
    const user = userEvent.setup();
    render(<PracticeWorkspace />);
    await submit(user);
    await user.click(await screen.findByRole("radio", { name: "opaque-a" }));
    await user.click(screen.getByRole("button", { name: /submit answer/i }));
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: /retry/i }));
    await screen.findByRole("region", { name: /answer feedback/i });
    expect(mockedCreateAttempt).toHaveBeenCalledTimes(1);
    expect(mockedSubmitAttempt).toHaveBeenCalledTimes(2);
  });
});
