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
}));

import { ApiError, listPracticeQuestions } from "./api-client";

const mockedList = vi.mocked(listPracticeQuestions);

beforeEach(() => {
  vi.clearAllMocks();
});

async function submit(user: ReturnType<typeof userEvent.setup>, syllabusId = "syl-1") {
  await user.type(screen.getByLabelText(/syllabus id/i), syllabusId);
  await user.click(screen.getByRole("button", { name: /load questions/i }));
}

describe("PracticeWorkspace", () => {
  it("shows nothing until the syllabus id is submitted", () => {
    render(<PracticeWorkspace />);
    expect(mockedList).not.toHaveBeenCalled();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("loads and renders eligible questions, forwarding an optional node filter", async () => {
    mockedList.mockResolvedValue({
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

    await user.type(screen.getByLabelText(/syllabus id/i), "syl-1");
    await user.type(screen.getByLabelText(/curriculum node id/i), "node-1");
    await user.click(screen.getByRole("button", { name: /load questions/i }));

    expect(await screen.findByText("opaque-prompt-1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "opaque-a" })).toBeInTheDocument();
    expect(mockedList).toHaveBeenCalledWith("syl-1", "node-1");
  });

  it("shows an empty state when no questions are eligible", async () => {
    mockedList.mockResolvedValue({ items: [] });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);

    await submit(user);

    expect(await screen.findByText(/no eligible questions yet/i)).toBeInTheDocument();
  });

  it("shows an error with retry, and retry calls the API again", async () => {
    mockedList.mockRejectedValueOnce(new ApiError(400, "unknown_syllabus", "syllabus is unknown or not active"));
    mockedList.mockResolvedValueOnce({ items: [] });
    const user = userEvent.setup();
    render(<PracticeWorkspace />);

    await submit(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(/syllabus is unknown or not active/i);

    await user.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));
  });

  it("lets a learner select an MCQ option without calling any API and never renders a submit/mark/reveal control", async () => {
    mockedList.mockResolvedValue({
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
    expect(mockedList).toHaveBeenCalledTimes(1); // selecting an option never triggers a fetch
    expect(screen.queryByRole("button", { name: /submit/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /mark/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reveal/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /start/i })).not.toBeInTheDocument();
  });
});
