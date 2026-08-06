"use client";

import { useCallback, useEffect, useState } from "react";
import { canReview } from "@/lib/editorial/permissions";
import sourceStyles from "../sources/styles.module.css";
import {
  apiErrorMessage,
  createQuestion,
  createRubricVersion,
  listQuestions,
  listRubricVersions,
  listSyllabuses,
  listVerifiedNodes,
  retireQuestion,
  setCanonicalRubric,
  updateQuestion,
  verifyQuestion,
  verifyRubricVersion,
} from "./api-client";
import { QuestionForm } from "./question-form";
import { QuestionList } from "./question-list";
import { QuestionReviewActions } from "./question-review-actions";
import { RubricWorkspace } from "./rubric-workspace";
import styles from "./styles.module.css";
import type {
  CreateQuestionRequest,
  CreateQuestionRubricVersionRequest,
  CurriculumMapNode,
  EditingRole,
  Question,
  QuestionRubricVersion,
  Syllabus,
  UpdateQuestionRequest,
} from "./types";

type Panel = { kind: "none" } | { kind: "create" } | { kind: "edit"; id: string };

export function QuestionsWorkspace({ role }: { role: EditingRole }) {
  const [syllabuses, setSyllabuses] = useState<Syllabus[] | null>(null);
  const [selectedSyllabusId, setSelectedSyllabusId] = useState<string | null>(null);
  const [nodes, setNodes] = useState<CurriculumMapNode[] | null>(null);
  const [nodeFilter, setNodeFilter] = useState("");
  const [questions, setQuestions] = useState<Question[] | null>(null);
  const [panel, setPanel] = useState<Panel>({ kind: "none" });
  const [versions, setVersions] = useState<QuestionRubricVersion[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [rubricLoadError, setRubricLoadError] = useState<string | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [mutating, setMutating] = useState(false);

  const loadContext = useCallback(async () => {
    setLoadError(null);
    try {
      const result = await listSyllabuses();
      setSyllabuses(result.items);
      setSelectedSyllabusId((current) => current ?? result.items[0]?.id ?? null);
    } catch (error) {
      setLoadError(apiErrorMessage(error));
      setSyllabuses((current) => current ?? []);
    }
  }, []);

  useEffect(() => { void loadContext(); }, [loadContext]);

  const loadSyllabus = useCallback(async (syllabusId: string, filter = "") => {
    setLoadError(null);
    setNodes(null);
    setQuestions(null);
    try {
      const [nodeResult, questionResult] = await Promise.all([
        listVerifiedNodes(syllabusId),
        listQuestions(syllabusId, filter || undefined),
      ]);
      setNodes(nodeResult.items.filter((node) => node.status === "verified"));
      setQuestions(questionResult.items);
    } catch (error) {
      setLoadError(apiErrorMessage(error));
      setNodes([]);
      setQuestions([]);
    }
  }, []);

  useEffect(() => {
    if (selectedSyllabusId) void loadSyllabus(selectedSyllabusId, nodeFilter);
  }, [selectedSyllabusId, nodeFilter, loadSyllabus]);

  const selected = panel.kind === "edit" ? questions?.find((question) => question.id === panel.id) ?? null : null;

  const loadVersions = useCallback(async (questionId: string) => {
    setRubricLoadError(null);
    setVersions(null);
    try {
      const result = await listRubricVersions(questionId);
      setVersions(result.items);
    } catch (error) {
      setRubricLoadError(apiErrorMessage(error));
      setVersions([]);
    }
  }, []);

  function selectQuestion(id: string) {
    setPanel({ kind: "edit", id });
    setMutationError(null);
    void loadVersions(id);
  }

  async function mutate<T>(action: () => Promise<T>, apply: (value: T) => void) {
    setMutating(true);
    setMutationError(null);
    try {
      apply(await action());
    } catch (error) {
      setMutationError(apiErrorMessage(error));
    } finally {
      setMutating(false);
    }
  }

  function replaceQuestion(updated: Question) {
    setQuestions((current) => (current ?? []).map((question) => question.id === updated.id ? updated : question));
  }

  async function handleCreate(input: CreateQuestionRequest) {
    await mutate(() => createQuestion(input), (created) => {
      setQuestions((current) => [created, ...(current ?? [])]);
      setPanel({ kind: "edit", id: created.id });
      setVersions([]);
    });
  }

  async function handleUpdate(id: string, input: UpdateQuestionRequest) {
    await mutate(() => updateQuestion(id, input), replaceQuestion);
  }

  async function handleCreateRubric(input: CreateQuestionRubricVersionRequest) {
    if (!selected) return;
    await mutate(() => createRubricVersion(selected.id, input), (created) => {
      setVersions((current) => [...(current ?? []), created]);
    });
  }

  async function handleVerifyRubric(version: number) {
    if (!selected) return;
    await mutate(() => verifyRubricVersion(selected.id, version), (updated) => {
      setVersions((current) => (current ?? []).map((item) => item.id === updated.id ? updated : item));
    });
  }

  function retryMain() {
    if (selectedSyllabusId) void loadSyllabus(selectedSyllabusId, nodeFilter);
    else void loadContext();
  }

  return <div className={sourceStyles.page}>
    <div className={sourceStyles.header}>
      <h1>Editorial questions</h1>
      {selectedSyllabusId && <button type="button" className={sourceStyles.button} onClick={() => {
        setPanel({ kind: "create" });
        setVersions(null);
        setMutationError(null);
      }}>New question</button>}
    </div>

    {syllabuses !== null && syllabuses.length > 0 && <div className={styles.pickerRow}>
      <div className={sourceStyles.field}>
        <label htmlFor="question-syllabus">Syllabus</label>
        <select id="question-syllabus" value={selectedSyllabusId ?? ""} onChange={(event) => {
          setSelectedSyllabusId(event.target.value);
          setNodeFilter("");
          setPanel({ kind: "none" });
          setVersions(null);
          setMutationError(null);
        }}>
          {syllabuses.map((syllabus) => <option key={syllabus.id} value={syllabus.id}>{syllabus.displayName} ({syllabus.syllabusCode})</option>)}
        </select>
      </div>
      <div className={sourceStyles.field}>
        <label htmlFor="question-node-filter">Verified node filter</label>
        <select id="question-node-filter" value={nodeFilter} onChange={(event) => {
          setNodeFilter(event.target.value);
          setPanel({ kind: "none" });
          setVersions(null);
        }}>
          <option value="">All verified nodes</option>
          {(nodes ?? []).map((node) => <option key={node.id} value={node.id}>{node.nodeCode} — {node.label}</option>)}
        </select>
      </div>
    </div>}

    {loadError && <p className={sourceStyles.bannerError} role="alert">{loadError} <button type="button" className={sourceStyles.buttonSecondary} onClick={retryMain}>Retry</button></p>}
    {syllabuses === null && !loadError && <p className={sourceStyles.loading} role="status">Loading…</p>}
    {syllabuses !== null && syllabuses.length === 0 && !loadError && <p className={sourceStyles.bannerEmpty} role="status">No active syllabuses are available.</p>}
    {selectedSyllabusId && (nodes === null || questions === null) && !loadError && <p className={sourceStyles.loading} role="status">Loading questions…</p>}
    {selectedSyllabusId && questions !== null && questions.length === 0 && !loadError && <p className={sourceStyles.bannerEmpty} role="status">No questions yet for current selection.</p>}
    {questions !== null && questions.length > 0 && <QuestionList questions={questions} nodes={nodes ?? []} selectedId={panel.kind === "edit" ? panel.id : null} onSelect={selectQuestion} />}

    {selectedSyllabusId && panel.kind === "create" && <QuestionForm
      key={`create-${selectedSyllabusId}`}
      mode={{ kind: "create" }}
      syllabusId={selectedSyllabusId}
      verifiedNodes={nodes ?? []}
      submitting={mutating}
      error={mutationError}
      onCreate={handleCreate}
      onUpdate={handleUpdate}
      onCancel={() => setPanel({ kind: "none" })}
    />}

    {selectedSyllabusId && selected && <>
      <QuestionForm
        key={`${selected.id}-${selected.contentRevision}`}
        mode={{ kind: "edit", question: selected }}
        syllabusId={selectedSyllabusId}
        verifiedNodes={nodes ?? []}
        submitting={mutating}
        error={mutationError}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
        onCancel={() => setPanel({ kind: "none" })}
      />
      <RubricWorkspace
        role={role}
        question={selected}
        versions={versions}
        loadingError={rubricLoadError}
        mutationError={mutationError}
        submitting={mutating}
        onRetry={() => void loadVersions(selected.id)}
        onCreate={handleCreateRubric}
        onVerify={handleVerifyRubric}
      />
      {canReview(role) && <QuestionReviewActions
        key={selected.id}
        question={selected}
        versions={versions}
        submitting={mutating}
        error={mutationError}
        onVerify={(id, rubricVersion) => void mutate(() => verifyQuestion(id, rubricVersion), replaceQuestion)}
        onSetCanonical={(id, rubricVersion) => void mutate(() => setCanonicalRubric(id, rubricVersion), replaceQuestion)}
        onRetire={(id) => void mutate(() => retireQuestion(id), replaceQuestion)}
      />}
    </>}
  </div>;
}
