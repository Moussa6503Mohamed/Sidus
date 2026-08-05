"use client";

import { useState, type FormEvent } from "react";
import { canReview } from "@/lib/editorial/permissions";
import sourceStyles from "../sources/styles.module.css";
import { validateRubricDraft, type RubricCriterionValues } from "./rubric-validation";
import styles from "./styles.module.css";
import type {
  CreateQuestionRubricVersionRequest,
  EditingRole,
  Question,
  QuestionRubricVersion,
} from "./types";

interface Props {
  role: EditingRole;
  question: Question;
  versions: QuestionRubricVersion[] | null;
  loadingError: string | null;
  mutationError: string | null;
  submitting: boolean;
  onRetry: () => void;
  onCreate: (input: CreateQuestionRubricVersionRequest) => void;
  onVerify: (version: number) => void;
}

export function RubricWorkspace(props: Props) {
  const { role, question, versions, loadingError, mutationError, submitting, onRetry, onCreate, onVerify } = props;
  const [criteria, setCriteria] = useState<RubricCriterionValues[]>([]);
  const [maxMarks, setMaxMarks] = useState("");
  const [correctOptionId, setCorrectOptionId] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [pendingVersion, setPendingVersion] = useState<number | null>(null);

  function addCriterion() {
    setCriteria((previous) => [...previous, { id: "", marks: "", descriptor: "" }]);
  }

  function setCriterion(index: number, field: keyof RubricCriterionValues, value: string) {
    setCriteria((previous) => previous.map((criterion, itemIndex) => itemIndex === index ? { ...criterion, [field]: value } : criterion));
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setValidationError(null);
    const result = validateRubricDraft(criteria, maxMarks, question, correctOptionId);
    if ("error" in result) return setValidationError(result.error);
    onCreate(result.input);
  }

  return (
    <section className={styles.workspace} aria-labelledby="rubric-heading">
      <h2 id="rubric-heading">Rubric versions</h2>
      {versions === null && !loadingError && <p className={sourceStyles.loading} role="status">Loading rubric versions…</p>}
      {loadingError && <p className={sourceStyles.bannerError} role="alert">{loadingError} <button type="button" className={sourceStyles.buttonSecondary} onClick={onRetry}>Retry</button></p>}
      {versions !== null && versions.length === 0 && !loadingError && <p className={sourceStyles.bannerEmpty} role="status">No rubric versions yet.</p>}
      {versions !== null && versions.length > 0 && <div className={sourceStyles.tableWrap}>
        <table>
          <thead><tr><th>Version</th><th>Status</th><th>Question revision</th><th>Marks</th><th>Freshness</th><th>Action</th></tr></thead>
          <tbody>{versions.map((version) => {
            const current = version.questionRevision === question.contentRevision;
            return <tr key={version.id}>
              <td>{version.version}</td>
              <td><span className={styles.badge} data-status={version.status}>{version.status}</span></td>
              <td>{version.questionRevision}</td>
              <td>{version.maxMarks}</td>
              <td><span className={styles.badge} data-status={current ? "current" : "stale"}>{current ? "current" : "stale"}</span></td>
              <td>{canReview(role) && version.status === "draft" ? <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setPendingVersion(version.version)} disabled={submitting}>Verify rubric</button> : "—"}</td>
            </tr>;
          })}</tbody>
        </table>
      </div>}

      {pendingVersion !== null && <div className={sourceStyles.confirm} role="alertdialog" aria-label="Confirm rubric verification">
        <p>Verify rubric version {pendingVersion}? Version content becomes immutable.</p>
        <div className={sourceStyles.formActions}>
          <button type="button" className={sourceStyles.button} onClick={() => { const version = pendingVersion; setPendingVersion(null); onVerify(version); }} disabled={submitting}>Confirm rubric verification</button>
          <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setPendingVersion(null)} disabled={submitting}>Cancel</button>
        </div>
      </div>}

      {question.status === "draft" ? <form className={sourceStyles.form} onSubmit={submit}>
        <fieldset disabled={submitting}>
          <legend>Create draft rubric version</legend>
          <div className={sourceStyles.field}>
            <label htmlFor="rubric-max-marks">Maximum marks (required)</label>
            <input id="rubric-max-marks" inputMode="numeric" value={maxMarks} onChange={(event) => setMaxMarks(event.target.value)} />
          </div>
          {question.responseType === "multiple_choice" && <div className={sourceStyles.field}>
            <label htmlFor="rubric-correct-option">Correct option (required)</label>
            <select id="rubric-correct-option" value={correctOptionId} onChange={(event) => setCorrectOptionId(event.target.value)} aria-required>
              <option value="">— select current option —</option>
              {(question.options ?? []).map((option) => <option key={option.id} value={option.id}>{option.id} — {option.label}</option>)}
            </select>
          </div>}
          <div className={styles.criteriaHeader}>
            <strong>Structured criteria</strong>
            <button type="button" className={sourceStyles.buttonSecondary} onClick={addCriterion}>Add criterion</button>
          </div>
          {criteria.length === 0 && <p className={sourceStyles.bannerEmpty} role="status">No criteria added.</p>}
          {criteria.map((criterion, index) => <div className={styles.criterion} key={index}>
            <div className={sourceStyles.grid}>
              <div className={sourceStyles.field}><label htmlFor={`criterion-id-${index}`}>Criterion id</label><input id={`criterion-id-${index}`} value={criterion.id} onChange={(event) => setCriterion(index, "id", event.target.value)} /></div>
              <div className={sourceStyles.field}><label htmlFor={`criterion-marks-${index}`}>Marks</label><input id={`criterion-marks-${index}`} inputMode="numeric" value={criterion.marks} onChange={(event) => setCriterion(index, "marks", event.target.value)} /></div>
              <div className={sourceStyles.field}><label htmlFor={`criterion-descriptor-${index}`}>Descriptor (optional)</label><input id={`criterion-descriptor-${index}`} value={criterion.descriptor} onChange={(event) => setCriterion(index, "descriptor", event.target.value)} /></div>
            </div>
            <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setCriteria((previous) => previous.filter((_, itemIndex) => itemIndex !== index))}>Remove criterion</button>
          </div>)}
        </fieldset>
        {validationError && <p className={sourceStyles.fieldError} role="alert">{validationError}</p>}
        {mutationError && <p className={sourceStyles.fieldError} role="alert">{mutationError}</p>}
        <button type="submit" className={sourceStyles.button} disabled={submitting}>Create rubric version</button>
      </form> : <p className={sourceStyles.bannerEmpty} role="status">New rubric versions require a draft question.</p>}
    </section>
  );
}
