import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { EditorialSourcesScreen } from "./editorial-sources-screen";

vi.mock("./api-client", () => ({
  ApiError: class ApiError extends Error {},
  listContentSources: vi.fn().mockResolvedValue({ items: [] }),
  listSyllabuses: vi.fn().mockResolvedValue({ items: [] }),
  createContentSource: vi.fn(),
  updateContentSource: vi.fn(),
  approveContentSource: vi.fn(),
  rejectContentSource: vi.fn(),
}));

import * as api from "./api-client";

describe.each(["learner", "unknown"] as const)("role=%s", (role) => {
  it("renders no editorial controls and never calls the API", () => {
    render(<EditorialSourcesScreen role={role} />);

    expect(screen.getByText(/no editorial access/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /new source/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(api.listContentSources).not.toHaveBeenCalled();
    expect(api.listSyllabuses).not.toHaveBeenCalled();
  });
});

describe.each(["editor", "reviewer", "admin"] as const)("role=%s", (role) => {
  it("renders the source workspace", async () => {
    render(<EditorialSourcesScreen role={role} />);

    expect(await screen.findByRole("heading", { name: /content sources/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /new source/i })).toBeInTheDocument();
  });
});
