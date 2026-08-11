"use client";

import { useEffect, useState } from "react";
import { ApiError, listPracticeModules, listPracticeQuestions, listPracticeSyllabuses } from "./api-client";
import { QuestionList } from "./question-list";
import styles from "./styles.module.css";
import type { LearnerModule, LearnerQuestion, LearnerSyllabus } from "./types";

type SyllabusState = { kind: "loading" } | { kind: "error"; message: string } | { kind: "empty" } | { kind: "loaded"; syllabuses: LearnerSyllabus[] };
type ModuleState = { kind: "idle" } | { kind: "loading" } | { kind: "error"; message: string } | { kind: "loaded"; modules: LearnerModule[] };
type QuestionState = { kind: "idle" } | { kind: "loading" } | { kind: "error"; message: string } | { kind: "loaded"; questions: LearnerQuestion[]; available: number };

function apiErrorMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : "Something went wrong. Please try again.";
}
function syllabusLabel(syllabus: LearnerSyllabus): string {
  return syllabus.track ? `${syllabus.displayName} (${syllabus.track})` : syllabus.displayName;
}
function moduleLabel(module: LearnerModule): string {
  return module.label ? `${module.code} — ${module.label}` : module.code;
}
function selectCount(items: LearnerQuestion[], count: string): LearnerQuestion[] | null {
  if (count === "all") return items;
  if (!/^\d+$/.test(count)) return null;
  const requested = Number(count);
  if (!Number.isSafeInteger(requested) || requested < 1 || requested > items.length) return null;
  return items.slice(0, requested);
}

export function PracticeWorkspace() {
  const [syllabusState, setSyllabusState] = useState<SyllabusState>({ kind: "loading" });
  const [moduleState, setModuleState] = useState<ModuleState>({ kind: "idle" });
  const [syllabusId, setSyllabusId] = useState("");
  const [moduleId, setModuleId] = useState("");
  const [count, setCount] = useState("all");
  const [questionState, setQuestionState] = useState<QuestionState>({ kind: "idle" });

  async function loadSyllabuses() {
    setSyllabusState({ kind: "loading" });
    try {
      const result = await listPracticeSyllabuses();
      setSyllabusState(result.items.length === 0 ? { kind: "empty" } : { kind: "loaded", syllabuses: result.items });
    } catch (err) { setSyllabusState({ kind: "error", message: apiErrorMessage(err) }); }
  }
  async function loadModules(syllabus: string) {
    setModuleState({ kind: "loading" });
    try {
      const result = await listPracticeModules(syllabus);
      setModuleState({ kind: "loaded", modules: result.items });
    } catch (err) { setModuleState({ kind: "error", message: apiErrorMessage(err) }); }
  }
  useEffect(() => { void loadSyllabuses(); }, []);

  function changeSyllabus(next: string) {
    setSyllabusId(next); setModuleId(""); setCount("all"); setQuestionState({ kind: "idle" });
    if (next) void loadModules(next); else setModuleState({ kind: "idle" });
  }
  async function loadQuestions() {
    setQuestionState({ kind: "loading" });
    try {
      const result = await listPracticeQuestions(syllabusId, moduleId || undefined);
      const selected = selectCount(result.items, count);
      if (!selected) {
        setQuestionState({ kind: "error", message: `Choose a whole number from 1 to ${result.items.length}, or All available questions.` });
        return;
      }
      setQuestionState({ kind: "loaded", questions: selected, available: result.items.length });
    } catch (err) { setQuestionState({ kind: "error", message: apiErrorMessage(err) }); }
  }
  function handleSubmit(event: React.FormEvent<HTMLFormElement>) { event.preventDefault(); if (syllabusId) void loadQuestions(); }

  return <div className={styles.page}>
    <div className={styles.header}><h1>Practice</h1><p>Choose module and question count. Submit one answer, then review verified feedback. No timer.</p></div>
    {syllabusState.kind === "loading" && <p className={styles.loading} role="status">Loading syllabuses…</p>}
    {syllabusState.kind === "error" && <p className={styles.bannerError} role="alert">{syllabusState.message} <button type="button" className={styles.buttonSecondary} onClick={() => void loadSyllabuses()}>Retry</button></p>}
    {syllabusState.kind === "empty" && <p className={styles.bannerEmpty} role="status">No active syllabuses are available yet.</p>}
    {syllabusState.kind === "loaded" && <form className={styles.form} onSubmit={handleSubmit}>
      <div className={styles.field}><label htmlFor="practice-syllabus-id">Syllabus</label><select id="practice-syllabus-id" value={syllabusId} onChange={(event) => changeSyllabus(event.target.value)} required><option value="" disabled>Select a syllabus</option>{syllabusState.syllabuses.map((syllabus) => <option key={syllabus.id} value={syllabus.id}>{syllabusLabel(syllabus)}</option>)}</select></div>
      {syllabusId && <div className={styles.field}><label htmlFor="practice-module">Module</label><select id="practice-module" value={moduleId} onChange={(event) => { setModuleId(event.target.value); setQuestionState({ kind: "idle" }); }} disabled={moduleState.kind === "loading" || moduleState.kind === "error"}><option value="">All modules</option>{moduleState.kind === "loaded" && moduleState.modules.map((module) => <option key={module.id} value={module.id}>{moduleLabel(module)}</option>)}</select>{moduleState.kind === "loading" && <span role="status">Loading modules…</span>}{moduleState.kind === "error" && <span role="alert">{moduleState.message} <button type="button" className={styles.buttonSecondary} onClick={() => void loadModules(syllabusId)}>Retry modules</button></span>}{moduleState.kind === "loaded" && moduleState.modules.length === 0 && <span>No modules with eligible questions yet.</span>}</div>}
      <div className={styles.field}><label htmlFor="practice-count">Questions</label><select id="practice-count" value={count === "all" ? "all" : "custom"} onChange={(event) => setCount(event.target.value === "all" ? "all" : count === "all" ? "1" : count)}><option value="all">All available</option><option value="custom">Choose a number</option></select>{count !== "all" && <input aria-label="Question count" type="number" min="1" step="1" value={count} onChange={(event) => setCount(event.target.value)} />}</div>
      <button type="submit" className={styles.button} disabled={questionState.kind === "loading" || !syllabusId || moduleState.kind === "loading"}>Load questions</button>
    </form>}
    {questionState.kind === "loading" && <p className={styles.loading} role="status">Loading questions…</p>}
    {questionState.kind === "error" && <p className={styles.bannerError} role="alert">{questionState.message} <button type="button" className={styles.buttonSecondary} onClick={() => void loadQuestions()}>Retry</button></p>}
    {questionState.kind === "loaded" && questionState.questions.length === 0 && <p className={styles.bannerEmpty} role="status">No eligible questions yet for this selection.</p>}
    {questionState.kind === "loaded" && questionState.questions.length > 0 && <><p role="status">Showing {questionState.questions.length} of {questionState.available} eligible questions.</p><QuestionList questions={questionState.questions} /></>}
  </div>;
}
