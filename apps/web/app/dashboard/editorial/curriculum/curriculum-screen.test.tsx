import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CurriculumMapScreen } from "./curriculum-screen";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  listSyllabuses: vi.fn().mockResolvedValue({ items: [] }),
  listApprovedContentSources: vi.fn().mockResolvedValue({ items: [] }),
  listCurriculumMapNodes: vi.fn().mockResolvedValue({ items: [] }),
  createCurriculumMapNode: vi.fn(),
  updateCurriculumMapNode: vi.fn(),
  verifyCurriculumMapNode: vi.fn(),
  retireCurriculumMapNode: vi.fn(),
}));

import * as api from "./api-client";

describe.each(["learner", "unknown"] as const)("role=%s", (role) => {
  it("renders no editorial controls and never calls the API", () => {
    render(<CurriculumMapScreen role={role} />);

    expect(screen.getByText(/no editorial access/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /new node/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(api.listSyllabuses).not.toHaveBeenCalled();
    expect(api.listApprovedContentSources).not.toHaveBeenCalled();
    expect(api.listCurriculumMapNodes).not.toHaveBeenCalled();
  });
});

describe.each(["editor", "reviewer", "admin"] as const)("role=%s", (role) => {
  it("renders the curriculum-map workspace", async () => {
    render(<CurriculumMapScreen role={role} />);

    expect(await screen.findByRole("heading", { name: /curriculum map/i })).toBeInTheDocument();
  });
});
