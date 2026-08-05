import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QuestionsWorkspace } from "./workspace";
import type { CurriculumMapNode, Question, QuestionRubricVersion, Syllabus } from "./types";

vi.mock("./api-client", async () => {
  const actual = await vi.importActual<typeof import("./api-client")>("./api-client");
  return {
    ...actual,
    listSyllabuses: vi.fn(),
    listVerifiedNodes: vi.fn(),
    listQuestions: vi.fn(),
    listRubricVersions: vi.fn(),
    createQuestion: vi.fn(),
    updateQuestion: vi.fn(),
    verifyQuestion: vi.fn(),
    retireQuestion: vi.fn(),
    createRubricVersion: vi.fn(),
    verifyRubricVersion: vi.fn(),
  };
});

import * as api from "./api-client";

const syllabus = {
  id: "syllabus-1",
  syllabusCode: "code-1",
  displayName: "Syllabus 1",
} as Syllabus;

const node = {
  id: "node-1",
  syllabusId: syllabus.id,
  nodeCode: "node-1",
  label: "Node 1",
  status: "verified",
} as CurriculumMapNode;

function question(overrides: Partial<Question> = {}): Question {
  return {
    id: "question-1",
    syllabusId: syllabus.id,
    curriculumMapNodeId: node.id,
    responseType: "short_answer",
    language: "lang-1",
    prompt: crypto.randomUUID(),
    status: "draft",
    contentRevision: 2,
    createdAt: "2026-08-05T00:00:00Z",
    updatedAt: "2026-08-05T00:00:00Z",
    ...overrides,
  };
}

function version(revision: number, status: "draft" | "verified", number: number): QuestionRubricVersion {
  return {
    id: `version-${number}`,
    questionId: "question-1",
    version: number,
    questionRevision: revision,
    rubric: { criteria: [] },
    maxMarks: 1,
    status,
    createdBy: "actor-1",
    reviewedBy: status === "verified" ? "actor-2" : null,
    createdAt: "2026-08-05T00:00:00Z",
    updatedAt: "2026-08-05T00:00:00Z",
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listSyllabuses).mockResolvedValue({ items: [syllabus] });
  vi.mocked(api.listVerifiedNodes).mockResolvedValue({ items: [node] });
  vi.mocked(api.listQuestions).mockResolvedValue({ items: [] });
  vi.mocked(api.listRubricVersions).mockResolvedValue({ items: [] });
});

describe("QuestionsWorkspace states", () => {
  it("shows loading then empty state", async () => {
    render(<QuestionsWorkspace role="editor" />);
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(await screen.findByText(/no questions yet/i)).toBeInTheDocument();
    expect(api.listQuestions).toHaveBeenCalledWith(syllabus.id, undefined);
    expect(api.listVerifiedNodes).toHaveBeenCalledWith(syllabus.id);
  });

  it("shows error and retries", async () => {
    vi.mocked(api.listSyllabuses).mockRejectedValueOnce(new Error("temporary"));
    render(<QuestionsWorkspace role="editor" />);
    expect(await screen.findByRole("alert")).toHaveTextContent(/something went wrong/i);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(await screen.findByLabelText("Syllabus")).toBeInTheDocument();
    expect(api.listSyllabuses).toHaveBeenCalledTimes(2);
  });
});

describe("QuestionsWorkspace editing and review", () => {
  it("creates draft from empty form using verified node", async () => {
    const runtimePrompt = crypto.randomUUID();
    const created = question({ prompt: runtimePrompt, contentRevision: 1 });
    vi.mocked(api.createQuestion).mockResolvedValue(created);
    render(<QuestionsWorkspace role="editor" />);
    await screen.findByText(/no questions yet/i);
    fireEvent.click(screen.getByRole("button", { name: /new question/i }));
    fireEvent.change(screen.getByLabelText(/verified curriculum node/i), { target: { value: node.id } });
    fireEvent.change(screen.getByLabelText(/response type/i), { target: { value: "short_answer" } });
    fireEvent.change(screen.getByLabelText(/language/i), { target: { value: "lang-1" } });
    fireEvent.change(screen.getByLabelText(/prompt/i), { target: { value: runtimePrompt } });
    fireEvent.click(screen.getByRole("button", { name: /create draft/i }));
    await waitFor(() => expect(api.createQuestion).toHaveBeenCalledWith({
      syllabusId: syllabus.id,
      curriculumMapNodeId: node.id,
      responseType: "short_answer",
      language: "lang-1",
      prompt: runtimePrompt,
    }));
  });

  it("edits draft with minimal patch and shows provenance warning", async () => {
    const draft = question();
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [draft] });
    vi.mocked(api.updateQuestion).mockResolvedValue({ ...draft, language: "lang-2", contentRevision: 3 });
    render(<QuestionsWorkspace role="editor" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    expect(screen.getByRole("note")).toHaveTextContent(/original editorial content only/i);
    fireEvent.change(screen.getByLabelText(/language/i), { target: { value: "lang-2" } });
    fireEvent.click(screen.getByRole("button", { name: /save draft/i }));
    await waitFor(() => expect(api.updateQuestion).toHaveBeenCalledWith(draft.id, { language: "lang-2" }));
  });

  it("presents stale and current rubric revisions", async () => {
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [question()] });
    vi.mocked(api.listRubricVersions).mockResolvedValue({ items: [version(1, "verified", 1), version(2, "draft", 2)] });
    render(<QuestionsWorkspace role="reviewer" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    expect(await screen.findByText("stale")).toBeInTheDocument();
    expect(screen.getByText("current")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /verify rubric/i })).toBeInTheDocument();
  });

  it("validates structured rubric before request", async () => {
    const draft = question();
    const criterionId = crypto.randomUUID();
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [draft] });
    vi.mocked(api.createRubricVersion).mockResolvedValue(version(2, "draft", 1));
    render(<QuestionsWorkspace role="editor" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    await screen.findByText(/no rubric versions yet/i);
    fireEvent.change(screen.getByLabelText(/maximum marks/i), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: /add criterion/i }));
    fireEvent.change(screen.getByLabelText("Criterion id"), { target: { value: criterionId } });
    fireEvent.change(screen.getByLabelText("Marks"), { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: /create rubric version/i }));
    expect(screen.getByRole("alert")).toHaveTextContent(/must equal sum/i);
    expect(api.createRubricVersion).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Marks"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: /create rubric version/i }));
    await waitFor(() => expect(api.createRubricVersion).toHaveBeenCalledWith(draft.id, {
      rubric: { criteria: [{ id: criterionId, marks: 2 }] },
      maxMarks: 2,
    }));
  });

  it("requires confirmation before question verification and retirement", async () => {
    const draft = question();
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [draft] });
    vi.mocked(api.verifyQuestion).mockResolvedValue({ ...draft, status: "verified" });
    vi.mocked(api.retireQuestion).mockResolvedValue({ ...draft, status: "retired" });
    render(<QuestionsWorkspace role="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    fireEvent.click(await screen.findByRole("button", { name: "Verify question" }));
    expect(api.verifyQuestion).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm verification" }));
    await waitFor(() => expect(api.verifyQuestion).toHaveBeenCalledWith(draft.id));
    fireEvent.click(await screen.findByRole("button", { name: "Retire question" }));
    expect(api.retireQuestion).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm retirement" }));
    await waitFor(() => expect(api.retireQuestion).toHaveBeenCalledWith(draft.id));
  });

  it("hides reviewer actions from editor", async () => {
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [question()] });
    render(<QuestionsWorkspace role="editor" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    expect(screen.queryByRole("button", { name: /verify question/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /retire question/i })).not.toBeInTheDocument();
  });
});
