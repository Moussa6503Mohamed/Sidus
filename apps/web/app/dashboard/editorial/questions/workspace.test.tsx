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
    setCanonicalRubric: vi.fn(),
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
    options: null,
    status: "draft",
    contentRevision: 2,
    createdAt: "2026-08-05T00:00:00Z",
    updatedAt: "2026-08-05T00:00:00Z",
    ...overrides,
    canonicalRubricVersionId: overrides.canonicalRubricVersionId ?? null,
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

  it("shows empty option editor only for MCQ and submits editor-authored options", async () => {
    const runtimePrompt = crypto.randomUUID();
    const optionOne = crypto.randomUUID();
    const optionTwo = crypto.randomUUID();
    const created = question({ responseType: "multiple_choice", prompt: runtimePrompt, options: [{ id: "one", label: optionOne }, { id: "two", label: optionTwo }], contentRevision: 1 });
    vi.mocked(api.createQuestion).mockResolvedValue(created);
    render(<QuestionsWorkspace role="editor" />);
    await screen.findByText(/no questions yet/i);
    fireEvent.click(screen.getByRole("button", { name: /new question/i }));
    expect(screen.queryByText(/multiple-choice options/i)).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/verified curriculum node/i), { target: { value: node.id } });
    fireEvent.change(screen.getByLabelText(/response type/i), { target: { value: "multiple_choice" } });
    expect(screen.getByText(/no options added/i)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/language/i), { target: { value: "lang-1" } });
    fireEvent.change(screen.getByLabelText(/prompt/i), { target: { value: runtimePrompt } });
    fireEvent.click(screen.getByRole("button", { name: /add option/i }));
    fireEvent.click(screen.getByRole("button", { name: /add option/i }));
    const ids = screen.getAllByLabelText("Option id");
    const labels = screen.getAllByLabelText("Original editorial label");
    expect(ids).toHaveLength(2);
    expect(ids[0]).toHaveValue("");
    expect(labels[0]).toHaveValue("");
    fireEvent.change(ids[0], { target: { value: "one" } });
    fireEvent.change(labels[0], { target: { value: optionOne } });
    fireEvent.change(ids[1], { target: { value: "two" } });
    fireEvent.change(labels[1], { target: { value: optionTwo } });
    fireEvent.click(screen.getByRole("button", { name: /create draft/i }));
    await waitFor(() => expect(api.createQuestion).toHaveBeenCalledWith(expect.objectContaining({
      responseType: "multiple_choice",
      options: [{ id: "one", label: optionOne }, { id: "two", label: optionTwo }],
    })));
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

  it("selects MCQ answer key only from current question options", async () => {
    const draft = question({ responseType: "multiple_choice", options: [{ id: "one", label: crypto.randomUUID() }, { id: "two", label: crypto.randomUUID() }] });
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [draft] });
    vi.mocked(api.createRubricVersion).mockResolvedValue({ ...version(2, "draft", 1), rubric: { criteria: [{ id: "c1", marks: 1 }], answerKey: { correctOptionId: "two" } } });
    render(<QuestionsWorkspace role="editor" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    await screen.findByText(/no rubric versions yet/i);
    fireEvent.change(screen.getByLabelText(/maximum marks/i), { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: /add criterion/i }));
    fireEvent.change(screen.getByLabelText("Criterion id"), { target: { value: "c1" } });
    fireEvent.change(screen.getByLabelText("Marks"), { target: { value: "1" } });
    const selector = screen.getByLabelText(/correct option/i);
    expect(selector.querySelectorAll("option")).toHaveLength(3);
    fireEvent.change(selector, { target: { value: "two" } });
    fireEvent.click(screen.getByRole("button", { name: /create rubric version/i }));
    await waitFor(() => expect(api.createRubricVersion).toHaveBeenCalledWith(draft.id, {
      rubric: { criteria: [{ id: "c1", marks: 1 }], answerKey: { correctOptionId: "two" } },
      maxMarks: 1,
    }));
  });

  it("disables option editing outside draft lifecycle", async () => {
    const verified = question({ status: "verified", responseType: "multiple_choice", options: [{ id: "one", label: crypto.randomUUID() }, { id: "two", label: crypto.randomUUID() }] });
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [verified] });
    render(<QuestionsWorkspace role="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    expect(screen.getAllByLabelText("Option id")[0]).toBeDisabled();
    expect(screen.getByRole("button", { name: /add option/i })).toBeDisabled();
  });

  it("requires confirmation before question verification and retirement", async () => {
    const draft = question();
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [draft] });
    const verifiedRubric = version(draft.contentRevision, "verified", 1);
    vi.mocked(api.listRubricVersions).mockResolvedValue({ items: [verifiedRubric] });
    vi.mocked(api.verifyQuestion).mockResolvedValue({ ...draft, status: "verified", canonicalRubricVersionId: verifiedRubric.id });
    vi.mocked(api.retireQuestion).mockResolvedValue({ ...draft, status: "retired" });
    render(<QuestionsWorkspace role="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    fireEvent.change(await screen.findByLabelText("Canonical rubric version"), { target: { value: "1" } });
    fireEvent.click(await screen.findByRole("button", { name: "Verify question" }));
    expect(api.verifyQuestion).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm verification" }));
    await waitFor(() => expect(api.verifyQuestion).toHaveBeenCalledWith(draft.id, 1));
    fireEvent.click(await screen.findByRole("button", { name: "Retire question" }));
    expect(api.retireQuestion).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm retirement" }));
    await waitFor(() => expect(api.retireQuestion).toHaveBeenCalledWith(draft.id));
  });

  it("offers only verified current rubrics and blocks empty selection", async () => {
    const draft = question();
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [draft] });
    vi.mocked(api.listRubricVersions).mockResolvedValue({ items: [
      version(draft.contentRevision - 1, "verified", 1),
      version(draft.contentRevision, "draft", 2),
    ] });
    render(<QuestionsWorkspace role="reviewer" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    expect(await screen.findByText(/no verified rubric exists for current/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Verify question" })).toBeDisabled();
    expect(screen.getByLabelText("Canonical rubric version")).toBeDisabled();
  });

  it("repairs historical verified null canonical and shows marker", async () => {
    const historical = question({ status: "verified", canonicalRubricVersionId: null });
    const selected = version(historical.contentRevision, "verified", 3);
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [historical] });
    vi.mocked(api.listRubricVersions).mockResolvedValue({ items: [selected] });
    vi.mocked(api.setCanonicalRubric).mockResolvedValue({ ...historical, canonicalRubricVersionId: selected.id });
    render(<QuestionsWorkspace role="admin" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    fireEvent.change(await screen.findByLabelText("Canonical rubric version"), { target: { value: "3" } });
    fireEvent.click(screen.getByRole("button", { name: "Set canonical rubric" }));
    expect(api.setCanonicalRubric).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm canonical selection" }));
    await waitFor(() => expect(api.setCanonicalRubric).toHaveBeenCalledWith(historical.id, 3));
    expect(await screen.findByText("canonical")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Set canonical rubric" })).not.toBeInTheDocument();
  });

  it("hides reviewer actions from editor", async () => {
    vi.mocked(api.listQuestions).mockResolvedValue({ items: [question()] });
    render(<QuestionsWorkspace role="editor" />);
    fireEvent.click(await screen.findByRole("button", { name: "Question 1" }));
    expect(screen.queryByRole("button", { name: /verify question/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /retire question/i })).not.toBeInTheDocument();
  });
});
