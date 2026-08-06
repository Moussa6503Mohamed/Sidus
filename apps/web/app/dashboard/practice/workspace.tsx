"use client";

import { useEffect, useState } from "react";
import { ApiError, listPracticeQuestions, listPracticeSyllabuses } from "./api-client";
import { QuestionList } from "./question-list";
import styles from "./styles.module.css";
import type { LearnerQuestion, LearnerSyllabus } from "./types";

type SyllabusState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { kind: "loaded"; syllabuses: LearnerSyllabus[] };

type QuestionState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "loaded"; questions: LearnerQuestion[] };

function apiErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Something went wrong. Please try again.";
}

function syllabusLabel(syllabus: LearnerSyllabus): string {
  return syllabus.track ? `${syllabus.displayName} (${syllabus.track})` : syllabus.displayName;
}

export function PracticeWorkspace() {
  const [syllabusState, setSyllabusState] = useState<SyllabusState>({ kind: "loading" });
  const [syllabusId, setSyllabusId] = useState("");
  const [nodeId, setNodeId] = useState("");
  const [questionState, setQuestionState] = useState<QuestionState>({ kind: "idle" });

  async function loadSyllabuses() {
    setSyllabusState({ kind: "loading" });
    try {
      const result = await listPracticeSyllabuses();
      setSyllabusState(
        result.items.length === 0 ? { kind: "empty" } : { kind: "loaded", syllabuses: result.items },
      );
    } catch (err) {
      setSyllabusState({ kind: "error", message: apiErrorMessage(err) });
    }
  }

  useEffect(() => {
    // Discovering which syllabuses exist is itself a safe, permission-gated read (D-0017 review
    // fix) — it runs on mount so the picker is never empty by default. Eligible *questions* are a
    // separate fetch that only ever runs after the learner explicitly selects a syllabus and
    // submits, never automatically.
    void loadSyllabuses();
  }, []);

  async function loadQuestions(syllabus: string, node: string) {
    setQuestionState({ kind: "loading" });
    try {
      const result = await listPracticeQuestions(syllabus, node || undefined);
      setQuestionState({ kind: "loaded", questions: result.items });
    } catch (err) {
      setQuestionState({ kind: "error", message: apiErrorMessage(err) });
    }
  }

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!syllabusId) return;
    void loadQuestions(syllabusId, nodeId.trim());
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1>Practice</h1>
        <p>Read verified questions for a syllabus. Nothing here is submitted, marked, or timed.</p>
      </div>

      {syllabusState.kind === "loading" && (
        <p className={styles.loading} role="status">
          Loading syllabuses…
        </p>
      )}

      {syllabusState.kind === "error" && (
        <p className={styles.bannerError} role="alert">
          {syllabusState.message}{" "}
          <button type="button" className={styles.buttonSecondary} onClick={() => void loadSyllabuses()}>
            Retry
          </button>
        </p>
      )}

      {syllabusState.kind === "empty" && (
        <p className={styles.bannerEmpty} role="status">
          No active syllabuses are available yet.
        </p>
      )}

      {syllabusState.kind === "loaded" && (
        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.field}>
            <label htmlFor="practice-syllabus-id">Syllabus</label>
            <select
              id="practice-syllabus-id"
              name="syllabusId"
              value={syllabusId}
              onChange={(event) => setSyllabusId(event.target.value)}
              required
            >
              <option value="" disabled>
                Select a syllabus
              </option>
              {syllabusState.syllabuses.map((syllabus) => (
                <option key={syllabus.id} value={syllabus.id}>
                  {syllabusLabel(syllabus)}
                </option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label htmlFor="practice-node-id">Curriculum node ID (temporary, developer-only, optional)</label>
            <input
              id="practice-node-id"
              name="curriculumMapNodeId"
              value={nodeId}
              onChange={(event) => setNodeId(event.target.value)}
            />
          </div>
          <button type="submit" className={styles.button} disabled={questionState.kind === "loading" || !syllabusId}>
            Load questions
          </button>
        </form>
      )}

      {questionState.kind === "loading" && (
        <p className={styles.loading} role="status">
          Loading questions…
        </p>
      )}

      {questionState.kind === "error" && (
        <p className={styles.bannerError} role="alert">
          {questionState.message}{" "}
          <button
            type="button"
            className={styles.buttonSecondary}
            onClick={() => void loadQuestions(syllabusId, nodeId.trim())}
          >
            Retry
          </button>
        </p>
      )}

      {questionState.kind === "loaded" && questionState.questions.length === 0 && (
        <p className={styles.bannerEmpty} role="status">
          No eligible questions yet for this syllabus.
        </p>
      )}

      {questionState.kind === "loaded" && questionState.questions.length > 0 && (
        <QuestionList questions={questionState.questions} />
      )}
    </div>
  );
}
