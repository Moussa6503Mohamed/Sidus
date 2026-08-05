"use client";

import { useState } from "react";
import sourceStyles from "../sources/styles.module.css";
import type { Question } from "./types";

interface Props {
  question: Question;
  submitting: boolean;
  error: string | null;
  onVerify: (id: string) => void;
  onRetire: (id: string) => void;
}

export function QuestionReviewActions({ question, submitting, error, onVerify, onRetire }: Props) {
  const [pending, setPending] = useState<"verify" | "retire" | null>(null);
  if (question.status === "retired") {
    return <section className={sourceStyles.reviewPanel}><h2>Question review</h2><p role="status">Question is retired. No review action is available.</p></section>;
  }
  return (
    <section className={sourceStyles.reviewPanel}>
      <h2>Question review</h2>
      {error && <p className={sourceStyles.fieldError} role="alert">{error}</p>}
      {pending === null && <div className={sourceStyles.formActions}>
        {question.status === "draft" && <button type="button" className={sourceStyles.button} onClick={() => setPending("verify")} disabled={submitting}>Verify question</button>}
        <button type="button" className={sourceStyles.buttonDanger} onClick={() => setPending("retire")} disabled={submitting}>Retire question</button>
      </div>}
      {pending === "verify" && <div className={sourceStyles.confirm} role="alertdialog" aria-label="Confirm question verification">
        <p>Verify this question at content revision {question.contentRevision}? This cannot be undone.</p>
        <div className={sourceStyles.formActions}>
          <button type="button" className={sourceStyles.button} onClick={() => { setPending(null); onVerify(question.id); }} disabled={submitting}>Confirm verification</button>
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
