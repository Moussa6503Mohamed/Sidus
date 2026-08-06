"use client";

import { useState } from "react";
import sourceStyles from "../sources/styles.module.css";
import type { Question, QuestionRubricVersion } from "./types";

interface Props {
  question: Question;
  versions: QuestionRubricVersion[] | null;
  submitting: boolean;
  error: string | null;
  onVerify: (id: string, rubricVersion: number) => void;
  onSetCanonical: (id: string, rubricVersion: number) => void;
  onRetire: (id: string) => void;
}

type Pending = "verify" | "canonical" | "retire" | null;

export function QuestionReviewActions(props: Props) {
  const { question, versions, submitting, error, onVerify, onSetCanonical, onRetire } = props;
  const [pending, setPending] = useState<Pending>(null);
  const [rubricVersion, setRubricVersion] = useState("");
  const eligible = (versions ?? []).filter((version) =>
    version.status === "verified" && version.questionRevision === question.contentRevision,
  );
  const selectedVersion = Number(rubricVersion);
  const selectionValid = eligible.some((version) => version.version === selectedVersion);
  const needsSelection = question.status === "draft" ||
    (question.status === "verified" && question.canonicalRubricVersionId === null);

  if (question.status === "retired") {
    return <section className={sourceStyles.reviewPanel}><h2>Question review</h2><p role="status">Question is retired. No review action is available.</p></section>;
  }

  function confirmSelection() {
    if (!selectionValid) return;
    const action = pending;
    setPending(null);
    if (action === "verify") onVerify(question.id, selectedVersion);
    if (action === "canonical") onSetCanonical(question.id, selectedVersion);
  }

  return (
    <section className={sourceStyles.reviewPanel}>
      <h2>Question review</h2>
      {error && <p className={sourceStyles.fieldError} role="alert">{error}</p>}

      {needsSelection && <div className={sourceStyles.field}>
        <label htmlFor={`canonical-rubric-${question.id}`}>Canonical rubric version</label>
        <select
          id={`canonical-rubric-${question.id}`}
          value={rubricVersion}
          onChange={(event) => setRubricVersion(event.target.value)}
          disabled={submitting || versions === null || eligible.length === 0}
        >
          <option value="">— select verified current rubric —</option>
          {eligible.map((version) => <option key={version.id} value={version.version}>Version {version.version}</option>)}
        </select>
        {versions === null && <p role="status">Loading eligible rubric versions…</p>}
        {versions !== null && eligible.length === 0 && <p role="status">No verified rubric exists for current question revision.</p>}
      </div>}

      {question.status === "verified" && question.canonicalRubricVersionId !== null &&
        <p role="status">Canonical rubric selected. Replacement is not allowed.</p>}

      {pending === null && <div className={sourceStyles.formActions}>
        {question.status === "draft" && <button type="button" className={sourceStyles.button} onClick={() => setPending("verify")} disabled={submitting || versions === null || !selectionValid}>Verify question</button>}
        {question.status === "verified" && question.canonicalRubricVersionId === null && <button type="button" className={sourceStyles.button} onClick={() => setPending("canonical")} disabled={submitting || versions === null || !selectionValid}>Set canonical rubric</button>}
        <button type="button" className={sourceStyles.buttonDanger} onClick={() => setPending("retire")} disabled={submitting}>Retire question</button>
      </div>}

      {(pending === "verify" || pending === "canonical") && <div className={sourceStyles.confirm} role="alertdialog" aria-label={pending === "verify" ? "Confirm question verification" : "Confirm canonical rubric selection"}>
        <p>{pending === "verify" ? "Verify" : "Repair"} this question using rubric version {selectedVersion}? Selection cannot be replaced.</p>
        <div className={sourceStyles.formActions}>
          <button type="button" className={sourceStyles.button} onClick={confirmSelection} disabled={submitting || !selectionValid}>{pending === "verify" ? "Confirm verification" : "Confirm canonical selection"}</button>
          <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setPending(null)} disabled={submitting}>Cancel</button>
        </div>
      </div>}

      {pending === "retire" && <div className={sourceStyles.confirm} role="alertdialog" aria-label="Confirm question retirement">
        <p>Retire this question? This cannot be undone.</p>
        <div className={sourceStyles.formActions}>
          <button type="button" className={sourceStyles.buttonDanger} onClick={() => { setPending(null); onRetire(question.id); }} disabled={submitting}>Confirm retirement</button>
          <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setPending(null)} disabled={submitting}>Cancel</button>
        </div>
      </div>}
    </section>
  );
}
