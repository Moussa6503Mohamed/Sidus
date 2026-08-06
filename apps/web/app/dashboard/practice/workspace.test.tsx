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
}));

import { ApiError, listPracticeQuestions, listPracticeSyllabuses } from "./api-client";

const mockedListQuestions = vi.mocked(listPracticeQuestions);
const mockedListSyllabuses = vi.mocked(listPracticeSyllabuses);

const oneActiveSyllabus = {
  items: [
    { id: "syl-1", board: "Cambridge", syllabusCode: "0610", qualification: "IGCSE", track: "Extended", displayName: "Biology 0610" },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  mockedListSyllabuses.mockResolvedValue(oneActiveSyllabus);
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
  it("loads and renders eligible questions for the selected syllabus, forwarding an optional node filter", async () => {
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
    await user.type(screen.getByLabelText(/curriculum node id/i), "node-1");
    await user.click(screen.getByRole("button", { name: /load questions/i }));

    expect(await screen.findByText("opaque-prompt-1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "opaque-a" })).toBeInTheDocument();
    expect(mockedListQuestions).toHaveBeenCalledWith("syl-1", "node-1");
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

  it("lets a learner select an MCQ option without calling any API and never renders a submit/mark/reveal control", async () => {
    mockedListQuestions.mockResolvedValue({
      items: [
        {
          id: "q-1",
          syllabusId: "syl-1",
          curriculumMapNodeId: "node-1",
          responseType: "multiple_choice",
          language: "en",
          prompt: "opaque-prompt-1",
          options: [{ id: "opt-a", label: "opaque-a" }],
          contentRevision: 1,
        },
      ],
    });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);
    await submit(user);
    await screen.findByText("opaque-prompt-1");

    const optionButton = screen.getByRole("button", { name: "opaque-a" });
    expect(optionButton).toHaveAttribute("aria-pressed", "false");

    await user.click(optionButton);

    expect(optionButton).toHaveAttribute("aria-pressed", "true");
    expect(mockedListQuestions).toHaveBeenCalledTimes(1); // selecting an option never triggers a fetch
    expect(screen.queryByRole("button", { name: /submit/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /mark/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reveal/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /start/i })).not.toBeInTheDocument();
  });
});
