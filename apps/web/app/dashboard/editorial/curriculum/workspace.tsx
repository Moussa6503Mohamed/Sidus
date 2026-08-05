"use client";

import { useCallback, useEffect, useState } from "react";
import { canReview } from "@/lib/editorial/permissions";
import sourceStyles from "../sources/styles.module.css";
import {
  ApiError,
  createCurriculumMapNode,
  listApprovedContentSources,
  listCurriculumMapNodes,
  listSyllabuses,
  retireCurriculumMapNode,
  updateCurriculumMapNode,
  verifyCurriculumMapNode,
} from "./api-client";
import { NodeForm } from "./node-form";
import { NodeList } from "./node-list";
import { ReviewActions } from "./review-actions";
import styles from "./styles.module.css";
import type {
  ContentSource,
  CreateCurriculumMapNodeRequest,
  CurriculumMapNode,
  EditingRole,
  Syllabus,
  UpdateCurriculumMapNodeRequest,
} from "./types";

interface WorkspaceProps {
  role: EditingRole;
}

type Panel = { kind: "none" } | { kind: "create" } | { kind: "edit"; id: string };

function apiErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Something went wrong. Please try again.";
}

export function CurriculumWorkspace({ role }: WorkspaceProps) {
  const [syllabuses, setSyllabuses] = useState<Syllabus[] | null>(null);
  const [contentSources, setContentSources] = useState<ContentSource[]>([]);
  const [selectedSyllabusId, setSelectedSyllabusId] = useState<string | null>(null);
  const [nodes, setNodes] = useState<CurriculumMapNode[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [panel, setPanel] = useState<Panel>({ kind: "none" });
  const [mutating, setMutating] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);

  const loadContext = useCallback(async () => {
    setLoadError(null);
    try {
      const [syllabusesResult, sourcesResult] = await Promise.all([listSyllabuses(), listApprovedContentSources()]);
      setSyllabuses(syllabusesResult.items);
      setContentSources(sourcesResult.items);
      setSelectedSyllabusId((prev) => prev ?? syllabusesResult.items[0]?.id ?? null);
    } catch (err) {
      setLoadError(apiErrorMessage(err));
      setSyllabuses((prev) => prev ?? []);
    }
  }, []);

  useEffect(() => {
    void loadContext();
  }, [loadContext]);

  const loadNodes = useCallback(async (syllabusId: string) => {
    setLoadError(null);
    setNodes(null);
    try {
      const result = await listCurriculumMapNodes(syllabusId);
      setNodes(result.items);
    } catch (err) {
      setLoadError(apiErrorMessage(err));
      setNodes([]);
    }
  }, []);

  useEffect(() => {
    if (selectedSyllabusId) void loadNodes(selectedSyllabusId);
  }, [selectedSyllabusId, loadNodes]);

  const selected = panel.kind === "edit" ? nodes?.find((n) => n.id === panel.id) ?? null : null;

  async function handleCreate(input: CreateCurriculumMapNodeRequest) {
    setMutating(true);
    setMutationError(null);
    try {
      const created = await createCurriculumMapNode(input);
      setNodes((prev) => [created, ...(prev ?? [])]);
      setPanel({ kind: "edit", id: created.id });
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function handleUpdate(id: string, input: UpdateCurriculumMapNodeRequest) {
    setMutating(true);
    setMutationError(null);
    try {
      const updated = await updateCurriculumMapNode(id, input);
      setNodes((prev) => (prev ?? []).map((n) => (n.id === id ? updated : n)));
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function handleVerify(id: string) {
    setMutating(true);
    setMutationError(null);
    try {
      const updated = await verifyCurriculumMapNode(id);
      setNodes((prev) => (prev ?? []).map((n) => (n.id === id ? updated : n)));
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function handleRetire(id: string) {
    setMutating(true);
    setMutationError(null);
    try {
      const updated = await retireCurriculumMapNode(id);
      setNodes((prev) => (prev ?? []).map((n) => (n.id === id ? updated : n)));
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  function retry() {
    if (selectedSyllabusId) void loadNodes(selectedSyllabusId);
    else void loadContext();
  }

  return (
    <div className={sourceStyles.page}>
      <div className={sourceStyles.header}>
        <h1>Curriculum map</h1>
        {selectedSyllabusId && (
          <button
            type="button"
            className={sourceStyles.button}
            onClick={() => {
              setMutationError(null);
              setPanel({ kind: "create" });
            }}
          >
            New node
          </button>
        )}
      </div>

      {syllabuses !== null && syllabuses.length > 0 && (
        <div className={styles.syllabusPicker}>
          <label htmlFor="syllabus-picker">Syllabus</label>
          <select
            id="syllabus-picker"
            value={selectedSyllabusId ?? ""}
            onChange={(e) => {
              setPanel({ kind: "none" });
              setMutationError(null);
              setSelectedSyllabusId(e.target.value);
            }}
          >
            {syllabuses.map((s) => (
              <option key={s.id} value={s.id}>
                {s.displayName} ({s.syllabusCode})
              </option>
            ))}
          </select>
        </div>
      )}

      {loadError && (
        <p className={sourceStyles.bannerError} role="alert">
          {loadError} <button type="button" className={sourceStyles.buttonSecondary} onClick={retry}>Retry</button>
        </p>
      )}

      {syllabuses === null && !loadError && (
        <p className={sourceStyles.loading} role="status">
          Loading…
        </p>
      )}

      {syllabuses !== null && syllabuses.length === 0 && !loadError && (
        <p className={sourceStyles.bannerEmpty} role="status">
          No active syllabuses are available yet.
        </p>
      )}

      {selectedSyllabusId && nodes === null && !loadError && (
        <p className={sourceStyles.loading} role="status">
          Loading nodes…
        </p>
      )}

      {selectedSyllabusId && nodes !== null && nodes.length === 0 && !loadError && (
        <p className={sourceStyles.bannerEmpty} role="status">
          No curriculum-map nodes yet for this syllabus. Use &ldquo;New node&rdquo; to create one.
        </p>
      )}

      {selectedSyllabusId && nodes !== null && nodes.length > 0 && (
        <NodeList
          nodes={nodes}
          contentSources={contentSources}
          selectedId={panel.kind === "edit" ? panel.id : null}
          onSelect={(id) => {
            setMutationError(null);
            setPanel({ kind: "edit", id });
          }}
        />
      )}

      {selectedSyllabusId && panel.kind === "create" && (
        <NodeForm
          mode={{ kind: "create" }}
          syllabusId={selectedSyllabusId}
          nodes={nodes ?? []}
          contentSources={contentSources}
          submitting={mutating}
          error={mutationError}
          onCreate={handleCreate}
          onUpdate={handleUpdate}
          onCancel={() => setPanel({ kind: "none" })}
        />
      )}

      {selectedSyllabusId && panel.kind === "edit" && selected && (
        <>
          <NodeForm
            mode={{ kind: "edit", node: selected }}
            syllabusId={selectedSyllabusId}
            nodes={nodes ?? []}
            contentSources={contentSources}
            submitting={mutating}
            error={mutationError}
            onCreate={handleCreate}
            onUpdate={handleUpdate}
            onCancel={() => setPanel({ kind: "none" })}
          />
          {canReview(role) && (
            <ReviewActions
              node={selected}
              submitting={mutating}
              error={mutationError}
              onVerify={handleVerify}
              onRetire={handleRetire}
            />
          )}
        </>
      )}
    </div>
  );
}
