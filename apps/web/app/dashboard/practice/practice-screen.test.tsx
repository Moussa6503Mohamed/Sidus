import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PracticeScreen } from "./practice-screen";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  listPracticeQuestions: vi.fn(),
  listPracticeSyllabuses: vi.fn(),
}));

import * as api from "./api-client";

const mockedListSyllabuses = vi.mocked(api.listPracticeSyllabuses);

beforeEach(() => {
  vi.clearAllMocks();
  mockedListSyllabuses.mockResolvedValue({
    items: [{ id: "syl-1", board: "Cambridge", syllabusCode: "0610", qualification: "IGCSE", track: null, displayName: "Biology 0610" }],
  });
});

describe("role=unknown", () => {
  it("renders no practice controls and makes zero learner BFF calls", () => {
    render(<PracticeScreen role="unknown" />);

    expect(screen.getByText(/no practice access/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /load questions/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /syllabus/i })).not.toBeInTheDocument();
    expect(api.listPracticeQuestions).not.toHaveBeenCalled();
    expect(api.listPracticeSyllabuses).not.toHaveBeenCalled();
  });
});

describe.each(["learner", "editor", "reviewer", "admin"] as const)("role=%s", (role) => {
  it("renders the practice workspace, discovers syllabuses, and does not call the questions API before submit", async () => {
    render(<PracticeScreen role={role} />);

    expect(screen.getByRole("heading", { name: /practice/i })).toBeInTheDocument();
    await screen.findByRole("combobox", { name: /syllabus/i });
    expect(api.listPracticeSyllabuses).toHaveBeenCalledTimes(1);
    expect(api.listPracticeQuestions).not.toHaveBeenCalled();
  });
});
