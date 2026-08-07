import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ContentSource, Syllabus } from "./types";
import { EditorialSourcesWorkspace } from "./workspace";

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
  listContentSources: vi.fn(),
  listSyllabuses: vi.fn(),
  createContentSource: vi.fn(),
  updateContentSource: vi.fn(),
  approveContentSource: vi.fn(),
  rejectContentSource: vi.fn(),
}));

import * as api from "./api-client";

const mockedListContentSources = vi.mocked(api.listContentSources);
const mockedListSyllabuses = vi.mocked(api.listSyllabuses);
const mockedCreate = vi.mocked(api.createContentSource);
const mockedUpdate = vi.mocked(api.updateContentSource);
const mockedApprove = vi.mocked(api.approveContentSource);
const mockedReject = vi.mocked(api.rejectContentSource);

const SYLLABUS: Syllabus = {
  id: "syl-1",
  board: "Cambridge",
  syllabusCode: "0610",
  subjectId: "subj-1",
  qualification: "IGCSE",
  track: "Extended",
  displayName: "IGCSE Biology 0610 Extended",
  curriculumYear: null,
  status: "active",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function makeSource(overrides: Partial<ContentSource> = {}): ContentSource {
  return {
    id: "src-1",
    title: "Cambridge IGCSE Biology coursebook",
    owner: null,
    sourceUrl: "https://example.org/biology",
    sourceHash: null,
    licenceReference: null,
    permittedUse: null,
    allowedAudience: null,
    syllabusCode: "0610",
    catalogueSyllabusId: null,
    status: "pending",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedListSyllabuses.mockResolvedValue({ items: [SYLLABUS] });
});

describe("loading, empty, and error states", () => {
  it("shows a loading state before data resolves", async () => {
    let resolveList: (v: { items: ContentSource[] }) => void = () => {};
    mockedListContentSources.mockReturnValue(
      new Promise((resolve) => {
        resolveList = resolve;
      }),
    );

    render(<EditorialSourcesWorkspace role="editor" />);

    expect(screen.getByText(/loading sources/i)).toBeInTheDocument();
    resolveList({ items: [] });
    await waitFor(() => expect(screen.queryByText(/loading sources/i)).not.toBeInTheDocument());
  });

  it("shows an empty state when there are no sources", async () => {
    mockedListContentSources.mockResolvedValue({ items: [] });

    render(<EditorialSourcesWorkspace role="editor" />);

    expect(await screen.findByText(/no content sources yet/i)).toBeInTheDocument();
  });

  it("shows an error banner with retry on load failure", async () => {
    const user = userEvent.setup();
    mockedListContentSources
      .mockRejectedValueOnce(new api.ApiError(502, "upstream_unavailable", "service is down"))
      .mockResolvedValueOnce({ items: [] });

    render(<EditorialSourcesWorkspace role="editor" />);

    expect(await screen.findByText(/service is down/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /retry/i }));

    expect(await screen.findByText(/no content sources yet/i)).toBeInTheDocument();
    expect(mockedListContentSources).toHaveBeenCalledTimes(2);
  });
});

describe("list rendering", () => {
  it("renders sources with status and syllabus code", async () => {
    mockedListContentSources.mockResolvedValue({ items: [makeSource()] });

    render(<EditorialSourcesWorkspace role="editor" />);

    expect(await screen.findByText(/cambridge igcse biology coursebook/i)).toBeInTheDocument();
    // Status label comes from the shared lib/design/status.ts map (StatusBadge), not a
    // page-local copy — "Pending review" matches the Sidus Observatory status vocabulary.
    expect(screen.getByText("Pending review")).toBeInTheDocument();
    expect(screen.getByText("0610")).toBeInTheDocument();
  });
});

describe("create flow", () => {
  it("creates a source with the minimum required fields", async () => {
    const user = userEvent.setup();
    mockedListContentSources.mockResolvedValue({ items: [] });
    mockedCreate.mockResolvedValue(makeSource({ id: "new-1", title: "New source" }));

    render(<EditorialSourcesWorkspace role="editor" />);
    await screen.findByText(/no content sources yet/i);

    await user.click(screen.getByRole("button", { name: /new source/i }));
    await user.type(screen.getByLabelText(/title/i), "New source");
    await user.type(screen.getByLabelText(/source url/i), "https://example.org/new");
    await user.click(screen.getByRole("button", { name: /create source/i }));

    await waitFor(() => expect(mockedCreate).toHaveBeenCalledWith({
      title: "New source",
      sourceUrl: "https://example.org/new",
    }));
  });

  it("blocks submission without title or source URL", async () => {
    const user = userEvent.setup();
    mockedListContentSources.mockResolvedValue({ items: [] });

    render(<EditorialSourcesWorkspace role="editor" />);
    await screen.findByText(/no content sources yet/i);
    await user.click(screen.getByRole("button", { name: /new source/i }));
    await user.click(screen.getByRole("button", { name: /create source/i }));

    expect(await screen.findByText(/title and source url are required/i)).toBeInTheDocument();
    expect(mockedCreate).not.toHaveBeenCalled();
  });

  // D-0021: the form shows help text for the private licensed-source reference URI and
  // forwards it to Core verbatim — the web layer performs no format validation of its own,
  // Core (D-0021) is the sole authority on the exact grammar.
  it("shows help text for the private licensed-source reference URI and submits it verbatim", async () => {
    const user = userEvent.setup();
    mockedListContentSources.mockResolvedValue({ items: [] });
    mockedCreate.mockResolvedValue(makeSource({ id: "new-2", title: "Licensed pair" }));

    render(<EditorialSourcesWorkspace role="editor" />);
    await screen.findByText(/no content sources yet/i);
    await user.click(screen.getByRole("button", { name: /new source/i }));

    expect(screen.getByText(/sidus-private:\/\/licensed\/cambridge-international\/9700/i)).toBeInTheDocument();

    const privateUri = "sidus-private://licensed/cambridge-international/9700/m17/12";
    await user.type(screen.getByLabelText(/title/i), "Licensed pair");
    await user.type(screen.getByLabelText(/source url/i), privateUri);
    await user.click(screen.getByRole("button", { name: /create source/i }));

    await waitFor(() => expect(mockedCreate).toHaveBeenCalledWith({
      title: "Licensed pair",
      sourceUrl: privateUri,
    }));
  });
});

describe("edit flow", () => {
  it("submits only the changed field as a patch", async () => {
    const user = userEvent.setup();
    const source = makeSource();
    mockedListContentSources.mockResolvedValue({ items: [source] });
    mockedUpdate.mockResolvedValue({ ...source, owner: "Cambridge Assessment" });

    render(<EditorialSourcesWorkspace role="editor" />);
    await user.click(await screen.findByRole("button", { name: /cambridge igcse biology coursebook/i }));

    const ownerInput = await screen.findByLabelText(/^owner$/i);
    await user.type(ownerInput, "Cambridge Assessment");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() =>
      expect(mockedUpdate).toHaveBeenCalledWith("src-1", { owner: "Cambridge Assessment" }),
    );
  });

  it("disables editing for a non-pending source", async () => {
    mockedListContentSources.mockResolvedValue({ items: [makeSource({ status: "approved" })] });

    render(<EditorialSourcesWorkspace role="editor" />);
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: /cambridge igcse biology coursebook/i }));

    expect(await screen.findByText(/can no longer be edited/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /save changes/i })).not.toBeInTheDocument();
  });
});

describe("review controls by role", () => {
  it("hides review controls for an editor", async () => {
    mockedListContentSources.mockResolvedValue({ items: [makeSource()] });
    const user = userEvent.setup();

    render(<EditorialSourcesWorkspace role="editor" />);
    await user.click(await screen.findByRole("button", { name: /cambridge igcse biology coursebook/i }));

    expect(screen.queryByRole("heading", { name: /review/i })).not.toBeInTheDocument();
  });

  it("shows approve/reject for a reviewer and runs the reject flow with confirmation", async () => {
    const user = userEvent.setup();
    const source = makeSource();
    mockedListContentSources.mockResolvedValue({ items: [source] });
    mockedReject.mockResolvedValue({ ...source, status: "rejected" });

    render(<EditorialSourcesWorkspace role="reviewer" />);
    await user.click(await screen.findByRole("button", { name: /cambridge igcse biology coursebook/i }));

    const reviewPanel = (await screen.findByRole("heading", { name: /review/i })).closest("div")!;
    await user.click(within(reviewPanel).getByRole("button", { name: /^reject$/i }));

    // Confirms without a reason first: blocked.
    await user.click(screen.getByRole("button", { name: /confirm rejection/i }));
    expect(await screen.findByText(/reason is required/i)).toBeInTheDocument();
    expect(mockedReject).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText(/reason/i), "Licence does not cover redistribution");
    await user.click(screen.getByRole("button", { name: /confirm rejection/i }));

    await waitFor(() =>
      expect(mockedReject).toHaveBeenCalledWith("src-1", "Licence does not cover redistribution"),
    );
  });

  it("disables approve when required approval fields are missing", async () => {
    mockedListContentSources.mockResolvedValue({ items: [makeSource()] });
    const user = userEvent.setup();

    render(<EditorialSourcesWorkspace role="admin" />);
    await user.click(await screen.findByRole("button", { name: /cambridge igcse biology coursebook/i }));

    expect(await screen.findByText(/missing before this source can be approved/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^approve$/i })).toBeDisabled();
  });
});
