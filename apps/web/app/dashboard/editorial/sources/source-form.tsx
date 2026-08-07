"use client";

import { useId, useState, type FormEvent } from "react";
import styles from "./styles.module.css";
import type { ContentSource, CreateContentSourceRequest, Syllabus, UpdateContentSourceRequest } from "./types";

interface FieldValues {
  title: string;
  sourceUrl: string;
  owner: string;
  sourceHash: string;
  licenceReference: string;
  permittedUse: string;
  allowedAudience: string;
  syllabusCode: string;
}

const EMPTY_VALUES: FieldValues = {
  title: "",
  sourceUrl: "",
  owner: "",
  sourceHash: "",
  licenceReference: "",
  permittedUse: "",
  allowedAudience: "",
  syllabusCode: "",
};

function valuesFromSource(source: ContentSource): FieldValues {
  return {
    title: source.title,
    sourceUrl: source.sourceUrl,
    owner: source.owner ?? "",
    sourceHash: source.sourceHash ?? "",
    licenceReference: source.licenceReference ?? "",
    permittedUse: source.permittedUse ?? "",
    allowedAudience: source.allowedAudience ?? "",
    syllabusCode: source.syllabusCode ?? "",
  };
}

type Mode =
  | { kind: "create" }
  | { kind: "edit"; source: ContentSource };

interface SourceFormProps {
  mode: Mode;
  syllabuses: Syllabus[];
  submitting: boolean;
  error: string | null;
  onCreate: (input: CreateContentSourceRequest) => void;
  onUpdate: (id: string, input: UpdateContentSourceRequest) => void;
  onCancel: () => void;
}

const FIELD_LABELS: Record<keyof Omit<FieldValues, "syllabusCode">, string> = {
  title: "Title",
  sourceUrl: "Source URL",
  owner: "Owner",
  sourceHash: "Source hash",
  licenceReference: "Licence reference",
  permittedUse: "Permitted use",
  allowedAudience: "Allowed audience",
};

export function SourceForm({ mode, syllabuses, submitting, error, onCreate, onUpdate, onCancel }: SourceFormProps) {
  const editable = mode.kind === "create" || mode.source.status === "pending";
  const [values, setValues] = useState<FieldValues>(
    mode.kind === "edit" ? valuesFromSource(mode.source) : EMPTY_VALUES,
  );
  const [validationError, setValidationError] = useState<string | null>(null);
  const headingId = useId();

  function setField(name: keyof FieldValues, value: string) {
    setValues((prev) => ({ ...prev, [name]: value }));
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setValidationError(null);

    if (mode.kind === "create") {
      if (values.title.trim() === "" || values.sourceUrl.trim() === "") {
        setValidationError("Title and source URL are required.");
        return;
      }
      const input: CreateContentSourceRequest = {
        title: values.title.trim(),
        sourceUrl: values.sourceUrl.trim(),
      };
      if (values.owner.trim()) input.owner = values.owner.trim();
      if (values.sourceHash.trim()) input.sourceHash = values.sourceHash.trim();
      if (values.licenceReference.trim()) input.licenceReference = values.licenceReference.trim();
      if (values.permittedUse.trim()) input.permittedUse = values.permittedUse.trim();
      if (values.allowedAudience.trim()) input.allowedAudience = values.allowedAudience.trim();
      if (values.syllabusCode.trim()) input.syllabusCode = values.syllabusCode.trim();
      onCreate(input);
      return;
    }

    const original = valuesFromSource(mode.source);
    const patch: UpdateContentSourceRequest = {};
    const clearedFields: string[] = [];
    (Object.keys(FIELD_LABELS) as Array<keyof typeof FIELD_LABELS>).forEach((key) => {
      const nextValue = values[key].trim();
      if (nextValue === original[key]) return;
      if (nextValue === "") {
        clearedFields.push(FIELD_LABELS[key]);
        return;
      }
      patch[key] = nextValue;
    });
    if (values.syllabusCode.trim() !== original.syllabusCode) {
      if (values.syllabusCode.trim() === "") {
        clearedFields.push("Syllabus");
      } else {
        patch.syllabusCode = values.syllabusCode.trim();
      }
    }

    if (clearedFields.length > 0) {
      setValidationError(`${clearedFields.join(", ")} cannot be cleared — supply a value or leave it unchanged.`);
      return;
    }
    if (Object.keys(patch).length === 0) {
      setValidationError("Change at least one field to update this source.");
      return;
    }
    onUpdate(mode.source.id, patch);
  }

  return (
    <form className={styles.form} onSubmit={handleSubmit} aria-labelledby={headingId}>
      <h2 id={headingId}>{mode.kind === "create" ? "New source" : `Edit ${mode.source.title || "source"}`}</h2>

      {!editable && (
        <p className={styles.bannerEmpty} role="status">
          This source is {mode.kind === "edit" ? mode.source.status : ""} and can no longer be edited.
        </p>
      )}

      <fieldset disabled={!editable || submitting}>
        <legend>Rights and provenance metadata</legend>
        <div className={styles.grid}>
          <div className={styles.field}>
            <label htmlFor="source-title">Title{mode.kind === "create" ? " (required)" : ""}</label>
            <input
              id="source-title"
              value={values.title}
              onChange={(e) => setField("title", e.target.value)}
              aria-required={mode.kind === "create"}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="source-url">Source URL{mode.kind === "create" ? " (required)" : ""}</label>
            <input
              id="source-url"
              type="text"
              inputMode="url"
              value={values.sourceUrl}
              onChange={(e) => setField("sourceUrl", e.target.value)}
              aria-required={mode.kind === "create"}
              aria-describedby="source-url-hint"
            />
            <p id="source-url-hint" className={styles.fieldHint}>
              An external http:// or https:// URL, or — for a licensed source with no public
              URL — the private reference
              sidus-private://licensed/cambridge-international/9700/{"{session}"}/{"{component}"}
              (session: m/s/w + two digits, e.g. m17; component: two digits, e.g. 12). This
              reference is metadata identity only; no file is uploaded, read, or linked here.
            </p>
          </div>
          <div className={styles.field}>
            <label htmlFor="source-owner">Owner</label>
            <input id="source-owner" value={values.owner} onChange={(e) => setField("owner", e.target.value)} />
          </div>
          <div className={styles.field}>
            <label htmlFor="source-hash">Source hash</label>
            <input
              id="source-hash"
              value={values.sourceHash}
              onChange={(e) => setField("sourceHash", e.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="source-licence">Licence reference</label>
            <input
              id="source-licence"
              value={values.licenceReference}
              onChange={(e) => setField("licenceReference", e.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="source-permitted-use">Permitted use</label>
            <input
              id="source-permitted-use"
              value={values.permittedUse}
              onChange={(e) => setField("permittedUse", e.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="source-audience">Allowed audience</label>
            <input
              id="source-audience"
              value={values.allowedAudience}
              onChange={(e) => setField("allowedAudience", e.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="source-syllabus">Syllabus</label>
            <select
              id="source-syllabus"
              value={values.syllabusCode}
              onChange={(e) => setField("syllabusCode", e.target.value)}
            >
              <option value="">— none —</option>
              {syllabuses.map((syllabus) => (
                <option key={syllabus.id} value={syllabus.syllabusCode}>
                  {syllabus.displayName} ({syllabus.syllabusCode})
                </option>
              ))}
            </select>
          </div>
        </div>
      </fieldset>

      {validationError && (
        <p className={styles.fieldError} role="alert">
          {validationError}
        </p>
      )}
      {error && (
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      )}

      <div className={styles.formActions}>
        {editable && (
          <button type="submit" className={styles.button} disabled={submitting}>
            {submitting ? "Saving…" : mode.kind === "create" ? "Create source" : "Save changes"}
          </button>
        )}
        <button type="button" className={styles.buttonSecondary} onClick={onCancel} disabled={submitting}>
          Cancel
        </button>
      </div>
    </form>
  );
}
