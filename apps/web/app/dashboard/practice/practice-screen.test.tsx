import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PracticeScreen } from "./practice-screen";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  listPracticeQuestions: vi.fn(),
}));

import * as api from "./api-client";

describe("role=unknown", () => {
  it("renders no practice controls and never calls the API", () => {
    render(<PracticeScreen role="unknown" />);

    expect(screen.getByText(/no practice access/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /load questions/i })).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/syllabus id/i)).not.toBeInTheDocument();
    expect(api.listPracticeQuestions).not.toHaveBeenCalled();
  });
});

describe.each(["learner", "editor", "reviewer", "admin"] as const)("role=%s", (role) => {
  it("renders the practice workspace and does not call the API before submit", () => {
    render(<PracticeScreen role={role} />);

    expect(screen.getByRole("heading", { name: /practice/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /load questions/i })).toBeInTheDocument();
    expect(api.listPracticeQuestions).not.toHaveBeenCalled();
  });
});
