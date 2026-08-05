"use client";

import { useId, useState, type FormEvent } from "react";
import sourceStyles from "../sources/styles.module.css";
import { buildCreateInput, buildUpdatePatch, EMPTY_NODE_VALUES, valuesFromNode, type NodeFieldValues } from "./node-form-diff";
import type {
  ContentSource,
  CreateCurriculumMapNodeRequest,
  CurriculumMapNode,
  CurriculumMapNodeKind,
  UpdateCurriculumMapNodeRequest,
} from "./types";

const NODE_KINDS: CurriculumMapNodeKind[] = ["topic", "objective", "practical_skill", "assessment_rule"];
const KIND_LABELS: Record<CurriculumMapNodeKind, string> = {
  topic: "Topic",
  objective: "Objective",
  practical_skill: "Practical skill",
  assessment_rule: "Assessment rule",
};

type Mode = { kind: "create" } | { kind: "edit"; node: CurriculumMapNode };

interface NodeFormProps {
  mode: Mode;
  syllabusId: string;
  nodes: CurriculumMapNode[];
  contentSources: ContentSource[];
  submitting: boolean;
  error: string | null;
  onCreate: (input: CreateCurriculumMapNodeRequest) => void;
  onUpdate: (id: string, input: UpdateCurriculumMapNodeRequest) => void;
  onCancel: () => void;
}

export function NodeForm({
  mode,
  syllabusId,
  nodes,
  contentSources,
  submitting,
  error,
  onCreate,
  onUpdate,
  onCancel,
}: NodeFormProps) {
  const editable = mode.kind === "create" || mode.node.status === "draft";
  const [values, setValues] = useState<NodeFieldValues>(
    mode.kind === "edit" ? valuesFromNode(mode.node) : EMPTY_NODE_VALUES,
  );
  const [validationError, setValidationError] = useState<string | null>(null);
  const headingId = useId();

  const eligibleSources = contentSources.filter((s) => s.catalogueSyllabusId === syllabusId);
  const eligibleParents = nodes.filter((n) => mode.kind === "create" || n.id !== mode.node.id);

  function setField<K extends keyof NodeFieldValues>(name: K, value: NodeFieldValues[K]) {
    setValues((prev) => ({ ...prev, [name]: value }));
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setValidationError(null);

    if (mode.kind === "create") {
      const result = buildCreateInput(syllabusId, values);
      if ("error" in result) {
        setValidationError(result.error);
        return;
      }
      onCreate(result.input);
      return;
    }

    const result = buildUpdatePatch(valuesFromNode(mode.node), values);
    if ("error" in result) {
      setValidationError(result.error);
      return;
    }
    onUpdate(mode.node.id, result.patch);
  }

  return (
    <form className={sourceStyles.form} onSubmit={handleSubmit} aria-labelledby={headingId}>
      <h2 id={headingId}>{mode.kind === "create" ? "New node" : `Edit ${mode.node.nodeCode || "node"}`}</h2>

      {!editable && (
        <p className={sourceStyles.bannerEmpty} role="status">
          This node is {mode.kind === "edit" ? mode.node.status : ""} and can no longer be edited.
        </p>
      )}

      <fieldset disabled={!editable || submitting}>
        <legend>Node metadata</legend>
        <div className={sourceStyles.grid}>
          <div className={sourceStyles.field}>
            <label htmlFor="node-kind">Kind (required)</label>
            <select id="node-kind" value={values.nodeKind} onChange={(e) => setField("nodeKind", e.target.value as CurriculumMapNodeKind)}>
              {NODE_KINDS.map((kind) => (
                <option key={kind} value={kind}>
                  {KIND_LABELS[kind]}
                </option>
              ))}
            </select>
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="node-code">Node code (required)</label>
            <input id="node-code" value={values.nodeCode} onChange={(e) => setField("nodeCode", e.target.value)} aria-required />
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="node-label">Label (required)</label>
            <input id="node-label" value={values.label} onChange={(e) => setField("label", e.target.value)} aria-required />
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="node-parent">Parent node</label>
            <select id="node-parent" value={values.parentNodeId} onChange={(e) => setField("parentNodeId", e.target.value)}>
              <option value="">— none (root) —</option>
              {eligibleParents.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.nodeCode} — {n.label || "(untitled)"} ({n.status})
                </option>
              ))}
            </select>
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="node-source">Content source (required)</label>
            <select id="node-source" value={values.contentSourceId} onChange={(e) => setField("contentSourceId", e.target.value)} aria-required>
              <option value="">— select an approved source —</option>
              {eligibleSources.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.title}
                </option>
              ))}
            </select>
            {eligibleSources.length === 0 && (
              <p className={sourceStyles.fieldError} role="status">
                No approved content source is linked to this syllabus yet — link one in Editorial
                sources first.
              </p>
            )}
          </div>
          <div className={sourceStyles.field}>
            <label htmlFor="node-locator">Source locator</label>
            <input id="node-locator" value={values.sourceLocator} onChange={(e) => setField("sourceLocator", e.target.value)} />
          </div>
        </div>
      </fieldset>

      {validationError && (
        <p className={sourceStyles.fieldError} role="alert">
          {validationError}
        </p>
      )}
      {error && (
        <p className={sourceStyles.fieldError} role="alert">
          {error}
        </p>
      )}

      <div className={sourceStyles.formActions}>
        {editable && (
          <button type="submit" className={sourceStyles.button} disabled={submitting}>
            {submitting ? "Saving…" : mode.kind === "create" ? "Create node" : "Save changes"}
          </button>
        )}
        <button type="button" className={sourceStyles.buttonSecondary} onClick={onCancel} disabled={submitting}>
          Cancel
        </button>
      </div>
    </form>
  );
}
