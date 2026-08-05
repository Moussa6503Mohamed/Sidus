import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EditorialQuestionsScreen } from "./editorial-questions-screen";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  apiErrorMessage: (error: unknown) => error instanceof Error ? error.message : "error",
  listSyllabuses: vi.fn().mockResolvedValue({ items: [] }),
  listVerifiedNodes: vi.fn(),
  listQuestions: vi.fn(),
  listRubricVersions: vi.fn(),
  createQuestion: vi.fn(),
  updateQuestion: vi.fn(),
  verifyQuestion: vi.fn(),
  retireQuestion: vi.fn(),
  createRubricVersion: vi.fn(),
  verifyRubricVersion: vi.fn(),
}));

import * as api from "./api-client";

beforeEach(() => vi.clearAllMocks());

describe.each(["learner", "unknown"] as const)("role=%s", (role) => {
  it("renders denied state and makes zero API calls", () => {
    render(<EditorialQuestionsScreen role={role} />);
    expect(screen.getByText(/no editorial access/i)).toBeInTheDocument();
    expect(api.listSyllabuses).not.toHaveBeenCalled();
    expect(api.listQuestions).not.toHaveBeenCalled();
    expect(api.listVerifiedNodes).not.toHaveBeenCalled();
  });
});

describe.each(["editor", "reviewer", "admin"] as const)("role=%s", (role) => {
  it("renders editorial workspace", async () => {
    render(<EditorialQuestionsScreen role={role} />);
    expect(await screen.findByRole("heading", { name: /editorial questions/i })).toBeInTheDocument();
  });
});
