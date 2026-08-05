"use client";

import { useId, useState, type FormEvent } from "react";
import sourceStyles from "../sources/styles.module.css";
import styles from "./styles.module.css";
import {
  buildCreateInput,
  buildUpdatePatch,
  EMPTY_QUESTION_VALUES,
  valuesFromQuestion,
  type QuestionFieldValues,
} from "./question-form-values";
import type {
  CreateQuestionRequest,
  CurriculumMapNode,
  Question,
  QuestionResponseType,
  UpdateQuestionRequest,
} from "./types";

const RESPONSE_TYPES: Array<{ value: QuestionResponseType; label: string }> = [
  { value: "multiple_choice", label: "Multiple choice" },
  { value: "short_answer", label: "Short answer" },
  { value: "structured_response", label: "Structured response" },
];

type Mode = { kind: "create" } | { kind: "edit"; question: Question };

interface QuestionFormProps {
  mode: Mode;
  syllabusId: string;
  verifiedNodes: CurriculumMapNode[];
  submitting: boolean;
  error: string | null;
  onCreate: (input: CreateQuestionRequest) => void;
  onUpdate: (id: string, input: UpdateQuestionRequest) => void;
  onCancel: () => void;
}

export function QuestionForm(props: QuestionFormProps) {
  const { mode, syllabusId, verifiedNodes, submitting, error, onCreate, onUpdate, onCancel } = props;
  const editable = mode.kind === "create" || mode.question.status === "draft";
  const [values, setValues] = useState<QuestionFieldValues>(
    mode.kind === "edit" ? valuesFromQuestion(mode.question) : EMPTY_QUESTION_VALUES,
  );
  const [validationError, setValidationError] = useState<string | null>(null);
  const headingId = useId();

  function setField<K extends keyof QuestionFieldValues>(name: K, value: QuestionFieldValues[K]) {
    setValues((previous) => ({ ...previous, [name]: value }));
  }

  function setOption(index: number, field: "id" | "label", value: string) {
    setValues((previous) => ({
      ...previous,
      options: previous.options.map((option, itemIndex) => itemIndex === index ? { ...option, [field]: value } : option),
    }));
  }

  function moveOption(index: number, direction: -1 | 1) {
    setValues((previous) => {
      const target = index + direction;
      if (target < 0 || target >= previous.options.length) return previous;
      const options = [...previous.options];
      [options[index], options[target]] = [options[target], options[index]];
      return { ...previous, options };
    });
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setValidationError(null);
    if (mode.kind === "create") {
      const result = buildCreateInput(syllabusId, values);
      if ("error" in result) return setValidationError(result.error);
      onCreate(result.input);
      return;
    }
    const result = buildUpdatePatch(valuesFromQuestion(mode.question), values);
    if ("error" in result) return setValidationError(result.error);
    onUpdate(mode.question.id, result.patch);
  }

  return (
    <form className={sourceStyles.form} onSubmit={submit} aria-labelledby={headingId}>
      <h2 id={headingId}>{mode.kind === "create" ? "New original question" : "Question draft"}</h2>
      <p className={sourceStyles.bannerEmpty} role="note">
        Original editorial content only. Do not copy or lightly rewrite source, syllabus, past-paper,
        mark-scheme, extract, or diagram content.
      </p>
      {!editable && (
        <p className={sourceStyles.bannerEmpty} role="status">
          This question is {mode.kind === "edit" ? mode.question.status : "locked"} and cannot be edited.
        </p>
      )}
      <fieldset disabled={!editable || submitting}>
        <legend>Question content</legend>
        <div className={sourceStyles.grid}>
          <div className={sourceStyles.field}>
            <label htmlFor="question-node">Verified curriculum node (required)</label>
            <select
              id="question-node"
              value={values.curriculumMapNodeId}
              onChange={(event) => setField("curriculumMapNodeId", event.target.value)}
              aria-required
            >
              <option value="">— select verified node —</option>
              {verifiedNodes.map((node) => (
                <option key={node.id} value={node.id}>
                  {node.nodeCode} — {node.label}
                </option>
              ))}
            </select>
            {verifiedNodes.length === 0 && (
              <p className={sourceStyles.fieldError} role="status">
                No verified curriculum nodes are available for this syllabus.
              </p>
            )}
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="question-response-type">Response type (required)</label>
            <select
              id="question-response-type"
              value={values.responseType}
              onChange={(event) => setField("responseType", event.target.value as QuestionFieldValues["responseType"])}
              aria-required
            >
              <option value="">— select response type —</option>
              {RESPONSE_TYPES.map((type) => <option key={type.value} value={type.value}>{type.label}</option>)}
            </select>
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="question-language">Language (required)</label>
            <input
              id="question-language"
              value={values.language}
              onChange={(event) => setField("language", event.target.value)}
              autoComplete="off"
              aria-required
            />
          </div>
        </div>
        <div className={sourceStyles.field}>
          <label htmlFor="question-prompt">Prompt (required)</label>
          <textarea
            id="question-prompt"
            rows={7}
            value={values.prompt}
            onChange={(event) => setField("prompt", event.target.value)}
            aria-required
          />
        </div>
        {values.responseType === "multiple_choice" && <section aria-labelledby={`${headingId}-options`}>
          <div className={styles.criteriaHeader}>
            <strong id={`${headingId}-options`}>Multiple-choice options (2–6 required)</strong>
            <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setField("options", [...values.options, { id: "", label: "" }])} disabled={values.options.length >= 6}>Add option</button>
          </div>
          {values.options.length === 0 && <p className={sourceStyles.bannerEmpty} role="status">No options added.</p>}
          {values.options.map((option, index) => <div className={styles.criterion} key={index}>
            <div className={sourceStyles.grid}>
              <div className={sourceStyles.field}><label htmlFor={`question-option-id-${index}`}>Option id</label><input id={`question-option-id-${index}`} value={option.id} onChange={(event) => setOption(index, "id", event.target.value)} /></div>
              <div className={sourceStyles.field}><label htmlFor={`question-option-label-${index}`}>Original editorial label</label><input id={`question-option-label-${index}`} value={option.label} onChange={(event) => setOption(index, "label", event.target.value)} /></div>
            </div>
            <div className={sourceStyles.formActions}>
              <button type="button" className={sourceStyles.buttonSecondary} onClick={() => moveOption(index, -1)} disabled={index === 0}>Move up</button>
              <button type="button" className={sourceStyles.buttonSecondary} onClick={() => moveOption(index, 1)} disabled={index === values.options.length - 1}>Move down</button>
              <button type="button" className={sourceStyles.buttonSecondary} onClick={() => setField("options", values.options.filter((_, itemIndex) => itemIndex !== index))}>Remove option</button>
            </div>
          </div>)}
        </section>}
      </fieldset>
      {validationError && <p className={sourceStyles.fieldError} role="alert">{validationError}</p>}
      {error && <p className={sourceStyles.fieldError} role="alert">{error}</p>}
      <div className={sourceStyles.formActions}>
        {editable && (
          <button type="submit" className={sourceStyles.button} disabled={submitting || verifiedNodes.length === 0}>
            {submitting ? "Saving…" : mode.kind === "create" ? "Create draft" : "Save draft"}
          </button>
        )}
        <button type="button" className={sourceStyles.buttonSecondary} onClick={onCancel} disabled={submitting}>Cancel</button>
      </div>
    </form>
  );
}
