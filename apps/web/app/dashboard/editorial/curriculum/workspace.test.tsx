import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ContentSource, CurriculumMapNode, Syllabus } from "./types";
import { CurriculumWorkspace } from "./workspace";

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
  listSyllabuses: vi.fn(),
  listApprovedContentSources: vi.fn(),
  listCurriculumMapNodes: vi.fn(),
  createCurriculumMapNode: vi.fn(),
  updateCurriculumMapNode: vi.fn(),
  verifyCurriculumMapNode: vi.fn(),
  retireCurriculumMapNode: vi.fn(),
}));

import * as api from "./api-client";

const mockedListSyllabuses = vi.mocked(api.listSyllabuses);
const mockedListSources = vi.mocked(api.listApprovedContentSources);
const mockedListNodes = vi.mocked(api.listCurriculumMapNodes);
const mockedCreate = vi.mocked(api.createCurriculumMapNode);
const mockedUpdate = vi.mocked(api.updateCurriculumMapNode);
const mockedVerify = vi.mocked(api.verifyCurriculumMapNode);
const mockedRetire = vi.mocked(api.retireCurriculumMapNode);

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
    owner: "Cambridge Assessment",
    sourceUrl: "https://example.org/biology",
    sourceHash: "hash",
    licenceReference: "licence",
    permittedUse: "use",
    allowedAudience: "audience",
    syllabusCode: "0610",
    catalogueSyllabusId: "syl-1",
    status: "approved",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function makeNode(overrides: Partial<CurriculumMapNode> = {}): CurriculumMapNode {
  return {
    id: "node-1",
    syllabusId: "syl-1",
    parentNodeId: null,
    nodeKind: "topic",
    nodeCode: "T1",
    label: "Cell biology",
    status: "draft",
    contentSourceId: "src-1",
    sourceLocator: null,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedListSyllabuses.mockResolvedValue({ items: [SYLLABUS] });
  mockedListSources.mockResolvedValue({ items: [makeSource()] });
});

describe("loading, empty, and error states", () => {
  it("shows a loading state before the syllabus and node data resolve", async () => {
    let resolveNodes: (v: { items: CurriculumMapNode[] }) => void = () => {};
    mockedListNodes.mockReturnValue(
      new Promise((resolve) => {
        resolveNodes = resolve;
      }),
    );

    render(<CurriculumWorkspace role="editor" />);

    expect(await screen.findByText(/loading nodes/i)).toBeInTheDocument();
    resolveNodes({ items: [] });
    await waitFor(() => expect(screen.queryByText(/loading nodes/i)).not.toBeInTheDocument());
  });

  it("shows an empty state when the syllabus has no nodes", async () => {
    mockedListNodes.mockResolvedValue({ items: [] });

    render(<CurriculumWorkspace role="editor" />);

    expect(await screen.findByText(/no curriculum-map nodes yet/i)).toBeInTheDocument();
  });

  it("shows an error banner with retry when the syllabus list fails to load", async () => {
    const user = userEvent.setup();
    mockedListSyllabuses
      .mockRejectedValueOnce(new api.ApiError(502, "upstream_unavailable", "service is down"))
      .mockResolvedValueOnce({ items: [SYLLABUS] });
    mockedListNodes.mockResolvedValue({ items: [] });

    render(<CurriculumWorkspace role="editor" />);

    expect(await screen.findByText(/service is down/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /retry/i }));

    expect(await screen.findByText(/no curriculum-map nodes yet/i)).toBeInTheDocument();
  });
});

describe("list rendering", () => {
  it("renders nodes with kind, status, parent hierarchy, and source", async () => {
    const parent = makeNode({ id: "node-1", nodeCode: "T1", label: "Cell biology" });
    const child = makeNode({
      id: "node-2",
      nodeCode: "T1.1",
      label: "Cell structure",
      parentNodeId: "node-1",
      status: "verified",
    });
    mockedListNodes.mockResolvedValue({ items: [parent, child] });

    render(<CurriculumWorkspace role="editor" />);

    expect(await screen.findByRole("button", { name: "T1.1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "T1" })).toBeInTheDocument();
    expect(screen.getByText("Draft")).toBeInTheDocument();
    expect(screen.getByText("Verified")).toBeInTheDocument();
    // The child row's Parent column shows the parent's nodeCode, alongside the parent's own row.
    expect(screen.getAllByText("T1").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText(/cambridge igcse biology coursebook/i).length).toBeGreaterThan(0);
  });
});

describe("create flow", () => {
  it("creates a node with the minimum required fields", async () => {
    const user = userEvent.setup();
    mockedListNodes.mockResolvedValue({ items: [] });
    mockedCreate.mockResolvedValue(makeNode({ id: "new-1", nodeCode: "T2" }));

    render(<CurriculumWorkspace role="editor" />);
    await screen.findByText(/no curriculum-map nodes yet/i);

    await user.click(screen.getByRole("button", { name: /new node/i }));
    await user.type(screen.getByLabelText(/node code/i), "T2");
    await user.type(screen.getByLabelText(/^label/i), "New topic");
    await user.selectOptions(screen.getByLabelText(/content source/i), "src-1");
    await user.click(screen.getByRole("button", { name: /create node/i }));

    await waitFor(() =>
      expect(mockedCreate).toHaveBeenCalledWith({
        syllabusId: "syl-1",
        nodeKind: "topic",
        nodeCode: "T2",
        label: "New topic",
        contentSourceId: "src-1",
      }),
    );
  });

  it("blocks submission without required fields", async () => {
    const user = userEvent.setup();
    mockedListNodes.mockResolvedValue({ items: [] });

    render(<CurriculumWorkspace role="editor" />);
    await screen.findByText(/no curriculum-map nodes yet/i);
    await user.click(screen.getByRole("button", { name: /new node/i }));
    await user.click(screen.getByRole("button", { name: /create node/i }));

    expect(await screen.findByText(/node code, label, and content source are required/i)).toBeInTheDocument();
    expect(mockedCreate).not.toHaveBeenCalled();
  });
});

describe("edit flow", () => {
  it("submits only the changed field as a patch", async () => {
    const user = userEvent.setup();
    const node = makeNode();
    mockedListNodes.mockResolvedValue({ items: [node] });
    mockedUpdate.mockResolvedValue({ ...node, label: "Updated label" });

    render(<CurriculumWorkspace role="editor" />);
    await user.click(await screen.findByRole("button", { name: "T1" }));

    const labelInput = await screen.findByLabelText(/^label/i);
    await user.clear(labelInput);
    await user.type(labelInput, "Updated label");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => expect(mockedUpdate).toHaveBeenCalledWith("node-1", { label: "Updated label" }));
  });

  it("disables editing for a non-draft node", async () => {
    mockedListNodes.mockResolvedValue({ items: [makeNode({ status: "verified" })] });

    render(<CurriculumWorkspace role="editor" />);
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "T1" }));

    expect(await screen.findByText(/can no longer be edited/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /save changes/i })).not.toBeInTheDocument();
  });
});

describe("review controls by role", () => {
  it("hides review controls for an editor", async () => {
    mockedListNodes.mockResolvedValue({ items: [makeNode()] });
    const user = userEvent.setup();

    render(<CurriculumWorkspace role="editor" />);
    await user.click(await screen.findByRole("button", { name: "T1" }));

    expect(screen.queryByRole("heading", { name: /review/i })).not.toBeInTheDocument();
  });

  it("shows verify for a reviewer and runs the confirm flow", async () => {
    const user = userEvent.setup();
    const node = makeNode();
    mockedListNodes.mockResolvedValue({ items: [node] });
    mockedVerify.mockResolvedValue({ ...node, status: "verified" });

    render(<CurriculumWorkspace role="reviewer" />);
    await user.click(await screen.findByRole("button", { name: "T1" }));

    const reviewPanel = (await screen.findByRole("heading", { name: /review/i })).closest("div")!;
    await user.click(within(reviewPanel).getByRole("button", { name: /^verify$/i }));
    await user.click(screen.getByRole("button", { name: /confirm verification/i }));

    await waitFor(() => expect(mockedVerify).toHaveBeenCalledWith("node-1"));
  });

  it("shows retire with confirmation for a verified node and does not offer verify again", async () => {
    const user = userEvent.setup();
    const node = makeNode({ status: "verified" });
    mockedListNodes.mockResolvedValue({ items: [node] });
    mockedRetire.mockResolvedValue({ ...node, status: "retired" });

    render(<CurriculumWorkspace role="admin" />);
    await user.click(await screen.findByRole("button", { name: "T1" }));

    const reviewPanel = (await screen.findByRole("heading", { name: /review/i })).closest("div")!;
    expect(within(reviewPanel).queryByRole("button", { name: /^verify$/i })).not.toBeInTheDocument();
    await user.click(within(reviewPanel).getByRole("button", { name: /^retire$/i }));
    await user.click(screen.getByRole("button", { name: /confirm retirement/i }));

    await waitFor(() => expect(mockedRetire).toHaveBeenCalledWith("node-1"));
  });

  it("offers no review action for an already-retired node", async () => {
    mockedListNodes.mockResolvedValue({ items: [makeNode({ status: "retired" })] });
    const user = userEvent.setup();

    render(<CurriculumWorkspace role="admin" />);
    await user.click(await screen.findByRole("button", { name: "T1" }));

    expect(await screen.findByText(/already retired/i)).toBeInTheDocument();
  });
});
