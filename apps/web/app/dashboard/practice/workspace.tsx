"use client";

import { useState } from "react";
import { ApiError, listPracticeQuestions } from "./api-client";
import { QuestionList } from "./question-list";
import styles from "./styles.module.css";
import type { LearnerQuestion } from "./types";

type LoadState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "loaded"; questions: LearnerQuestion[] };

function apiErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Something went wrong. Please try again.";
}

export function PracticeWorkspace() {
  const [syllabusId, setSyllabusId] = useState("");
  const [nodeId, setNodeId] = useState("");
  const [state, setState] = useState<LoadState>({ kind: "idle" });

  async function load(syllabus: string, node: string) {
    setState({ kind: "loading" });
    try {
      const result = await listPracticeQuestions(syllabus, node || undefined);
      setState({ kind: "loaded", questions: result.items });
    } catch (err) {
      setState({ kind: "error", message: apiErrorMessage(err) });
    }
  }

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedSyllabus = syllabusId.trim();
    if (!trimmedSyllabus) return;
    void load(trimmedSyllabus, nodeId.trim());
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1>Practice</h1>
        <p>Read verified questions for a syllabus. Nothing here is submitted, marked, or timed.</p>
      </div>

      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.field}>
          <label htmlFor="practice-syllabus-id">Syllabus ID</label>
          <input
            id="practice-syllabus-id"
            name="syllabusId"
            value={syllabusId}
            onChange={(event) => setSyllabusId(event.target.value)}
            required
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="practice-node-id">Curriculum node ID (optional)</label>
          <input
            id="practice-node-id"
            name="curriculumMapNodeId"
            value={nodeId}
            onChange={(event) => setNodeId(event.target.value)}
          />
        </div>
        <button type="submit" className={styles.button} disabled={state.kind === "loading" || !syllabusId.trim()}>
          Load questions
        </button>
      </form>

      {state.kind === "loading" && (
        <p className={styles.loading} role="status">
          Loading questions…
        </p>
      )}

      {state.kind === "error" && (
        <p className={styles.bannerError} role="alert">
          {state.message}{" "}
          <button
            type="button"
            className={styles.buttonSecondary}
            onClick={() => void load(syllabusId.trim(), nodeId.trim())}
          >
            Retry
          </button>
        </p>
      )}

      {state.kind === "loaded" && state.questions.length === 0 && (
        <p className={styles.bannerEmpty} role="status">
          No eligible questions yet for this syllabus.
        </p>
      )}

      {state.kind === "loaded" && state.questions.length > 0 && <QuestionList questions={state.questions} />}
    </div>
  );
}
