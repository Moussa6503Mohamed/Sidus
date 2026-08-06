import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QuestionList } from "./question-list";
import type { LearnerQuestion } from "./types";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  createPracticeAttempt: vi.fn(),
  submitPracticeAttempt: vi.fn(),
}));

import * as api from "./api-client";

const mockedCreateAttempt = vi.mocked(api.createPracticeAttempt);
const mockedSubmitAttempt = vi.mocked(api.submitPracticeAttempt);

const question: LearnerQuestion = {
  id: "q-1",
  syllabusId: "syl-1",
  curriculumMapNodeId: "node-1",
  responseType: "multiple_choice",
  language: "en",
  prompt: "opaque-prompt",
  options: [
    { id: "opt-a", label: "opaque-a" },
    { id: "opt-b", label: "opaque-b" },
    { id: "opt-c", label: "opaque-c" },
  ],
  contentRevision: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
  mockedCreateAttempt.mockResolvedValue({ attemptId: "attempt-1", questionId: "q-1", status: "open", maxMarks: 1 });
  mockedSubmitAttempt.mockResolvedValue({
    attemptId: "attempt-1",
    questionId: "q-1",
    selectedOptionId: "opt-b",
    correctOptionId: "opt-a",
    isCorrect: false,
    awardedMarks: 0,
    maxMarks: 1,
    feedback: { correctExplanation: "opaque-correct", incorrectExplanations: [{ optionId: "opt-b", explanation: "opaque-wrong" }] },
  });
});

function radios() {
  return screen.getAllByRole("radio");
}

describe("QuestionList — radiogroup labelling", () => {
  it("labels the radiogroup with the question prompt", () => {
    render(<QuestionList questions={[question]} />);

    const group = screen.getByRole("radiogroup");
    expect(group).toHaveAccessibleName("opaque-prompt");
  });
});

describe("QuestionList — roving tabIndex", () => {
  it("puts the roving tab stop on the first option when nothing is selected", () => {
    render(<QuestionList questions={[question]} />);

    const [first, second, third] = radios();
    expect(first).toHaveAttribute("tabIndex", "0");
    expect(second).toHaveAttribute("tabIndex", "-1");
    expect(third).toHaveAttribute("tabIndex", "-1");
  });

  it("moves the roving tab stop to the selected option on click", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);

    await user.click(radios()[1]);

    const [first, second, third] = radios();
    expect(first).toHaveAttribute("tabIndex", "-1");
    expect(second).toHaveAttribute("tabIndex", "0");
    expect(third).toHaveAttribute("tabIndex", "-1");
  });
});

describe("QuestionList — keyboard navigation", () => {
  it("ArrowRight/ArrowDown move to the next option and wrap at the end", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);
    const [first, second, third] = radios();

    first.focus();
    await user.keyboard("{ArrowRight}");
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute("aria-checked", "true");

    await user.keyboard("{ArrowDown}");
    expect(third).toHaveFocus();
    expect(third).toHaveAttribute("aria-checked", "true");

    await user.keyboard("{ArrowDown}");
    expect(first).toHaveFocus();
    expect(first).toHaveAttribute("aria-checked", "true");
  });

  it("ArrowLeft/ArrowUp move to the previous option and wrap at the start", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);
    const [first, second, third] = radios();

    first.focus();
    await user.keyboard("{ArrowLeft}");
    expect(third).toHaveFocus();
    expect(third).toHaveAttribute("aria-checked", "true");

    await user.keyboard("{ArrowUp}");
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute("aria-checked", "true");
  });

  it("Home moves to the first option and End moves to the last", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);
    const [first, , third] = radios();

    first.focus();
    await user.keyboard("{End}");
    expect(third).toHaveFocus();
    expect(third).toHaveAttribute("aria-checked", "true");

    await user.keyboard("{Home}");
    expect(first).toHaveFocus();
    expect(first).toHaveAttribute("aria-checked", "true");
  });

  it("Space and Enter select the already-focused option without moving focus", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);
    const [first, second] = radios();

    second.focus();
    await user.keyboard(" ");
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute("aria-checked", "true");

    first.focus();
    await user.keyboard("{Enter}");
    expect(first).toHaveFocus();
    expect(first).toHaveAttribute("aria-checked", "true");
  });

  it("Tab enters the group once at the roving tab stop and leaves it once at the Submit button", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);
    const [first, , third] = radios();

    await user.tab();
    expect(first).toHaveFocus();

    await user.keyboard("{End}");
    expect(third).toHaveFocus();

    await user.tab();
    expect(screen.getByRole("button", { name: /submit answer/i })).toHaveFocus();
  });
});

describe("QuestionList — no answer revealed before submit", () => {
  it("shows no correctness tag, feedback, or correct-answer text before submission", () => {
    render(<QuestionList questions={[question]} />);

    for (const radio of radios()) {
      expect(radio).not.toHaveAttribute("data-option-state", "correct");
      expect(radio).not.toHaveAttribute("data-option-state", "incorrect");
    }
    expect(screen.queryByRole("region", { name: /answer feedback/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/correct answer/i)).not.toBeInTheDocument();
  });
});

describe("QuestionList — read-only result semantics", () => {
  it("switches to a read-only list, disables options, and stops presenting them as a radiogroup", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);

    await user.click(radios()[1]);
    await user.click(screen.getByRole("button", { name: /submit answer/i }));

    await screen.findByRole("region", { name: /answer feedback/i });

    expect(screen.queryByRole("radiogroup")).not.toBeInTheDocument();
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    const group = screen.getByRole("list", { name: /options, marked/i });
    for (const item of within(group).getAllByRole("listitem")) {
      expect(item).toBeDisabled();
    }
    // Non-color-only correctness labels survive the read-only transition.
    expect(screen.getByText("Correct answer")).toBeInTheDocument();
    expect(screen.getByText("Your answer · incorrect")).toBeInTheDocument();
  });

  it("does not respond to arrow keys once marked", async () => {
    const user = userEvent.setup();
    render(<QuestionList questions={[question]} />);

    await user.click(radios()[1]);
    await user.click(screen.getByRole("button", { name: /submit answer/i }));
    await screen.findByRole("region", { name: /answer feedback/i });

    const group = screen.getByRole("list", { name: /options, marked/i });
    for (const item of within(group).getAllByRole("listitem")) {
      expect(item).toBeDisabled();
    }
  });
});
