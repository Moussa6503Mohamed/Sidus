"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  approveContentSource,
  createContentSource,
  listContentSources,
  listSyllabuses,
  rejectContentSource,
  updateContentSource,
} from "./api-client";
import { ReviewActions } from "./review-actions";
import { SourceForm } from "./source-form";
import { SourceList } from "./source-list";
import styles from "./styles.module.css";
import type { ContentSource, CreateContentSourceRequest, EditingRole, Syllabus, UpdateContentSourceRequest } from "./types";
import { canReview } from "@/lib/editorial/permissions";

interface WorkspaceProps {
  role: EditingRole;
}

type Panel = { kind: "none" } | { kind: "create" } | { kind: "edit"; id: string };

function apiErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Something went wrong. Please try again.";
}

export function EditorialSourcesWorkspace({ role }: WorkspaceProps) {
  const [sources, setSources] = useState<ContentSource[] | null>(null);
  const [syllabuses, setSyllabuses] = useState<Syllabus[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [panel, setPanel] = useState<Panel>({ kind: "none" });
  const [mutating, setMutating] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);

  const loadAll = useCallback(async () => {
    setLoadError(null);
    try {
      const [sourcesResult, syllabusesResult] = await Promise.all([listContentSources(), listSyllabuses()]);
      setSources(sourcesResult.items);
      setSyllabuses(syllabusesResult.items);
    } catch (err) {
      setLoadError(apiErrorMessage(err));
      setSources((prev) => prev ?? []);
    }
  }, []);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const selected = panel.kind === "edit" ? sources?.find((s) => s.id === panel.id) ?? null : null;

  async function handleCreate(input: CreateContentSourceRequest) {
    setMutating(true);
    setMutationError(null);
    try {
      const created = await createContentSource(input);
      setSources((prev) => [created, ...(prev ?? [])]);
      setPanel({ kind: "edit", id: created.id });
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function handleUpdate(id: string, input: UpdateContentSourceRequest) {
    setMutating(true);
    setMutationError(null);
    try {
      const updated = await updateContentSource(id, input);
      setSources((prev) => (prev ?? []).map((s) => (s.id === id ? updated : s)));
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function handleApprove(id: string) {
    setMutating(true);
    setMutationError(null);
    try {
      const updated = await approveContentSource(id);
      setSources((prev) => (prev ?? []).map((s) => (s.id === id ? updated : s)));
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function handleReject(id: string, reason: string) {
    setMutating(true);
    setMutationError(null);
    try {
      const updated = await rejectContentSource(id, reason);
      setSources((prev) => (prev ?? []).map((s) => (s.id === id ? updated : s)));
    } catch (err) {
      setMutationError(apiErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1>Content sources</h1>
        <button
          type="button"
          className={styles.button}
          onClick={() => {
            setMutationError(null);
            setPanel({ kind: "create" });
          }}
        >
          New source
        </button>
      </div>

      {loadError && (
        <p className={styles.bannerError} role="alert">
          {loadError}{" "}
          <button type="button" className={styles.buttonSecondary} onClick={() => void loadAll()}>
            Retry
          </button>
        </p>
      )}

      {sources === null && !loadError && (
        <p className={styles.loading} role="status">
          Loading sources…
        </p>
      )}

      {sources !== null && sources.length === 0 && !loadError && (
        <p className={styles.bannerEmpty} role="status">
          No content sources yet. Use &ldquo;New source&rdquo; to register one.
        </p>
      )}

      {sources !== null && sources.length > 0 && (
        <SourceList
          sources={sources}
          selectedId={panel.kind === "edit" ? panel.id : null}
          onSelect={(id) => {
            setMutationError(null);
            setPanel({ kind: "edit", id });
          }}
        />
      )}

      {panel.kind === "create" && (
        <SourceForm
          mode={{ kind: "create" }}
          syllabuses={syllabuses}
          submitting={mutating}
          error={mutationError}
          onCreate={handleCreate}
          onUpdate={handleUpdate}
          onCancel={() => setPanel({ kind: "none" })}
        />
      )}

      {panel.kind === "edit" && selected && (
        <>
          <SourceForm
            mode={{ kind: "edit", source: selected }}
            syllabuses={syllabuses}
            submitting={mutating}
            error={mutationError}
            onCreate={handleCreate}
            onUpdate={handleUpdate}
            onCancel={() => setPanel({ kind: "none" })}
          />
          {canReview(role) && (
            <ReviewActions
              source={selected}
              submitting={mutating}
              error={mutationError}
              onApprove={handleApprove}
              onReject={handleReject}
            />
          )}
        </>
      )}
    </div>
  );
}
