"use client";

import { useState } from "react";
import sourceStyles from "../sources/styles.module.css";
import type { CurriculumMapNode } from "./types";

interface ReviewActionsProps {
  node: CurriculumMapNode;
  submitting: boolean;
  error: string | null;
  onVerify: (id: string) => void;
  onRetire: (id: string) => void;
}

type PendingAction = "verify" | "retire" | null;

export function ReviewActions({ node, submitting, error, onVerify, onRetire }: ReviewActionsProps) {
  const [pendingAction, setPendingAction] = useState<PendingAction>(null);

  if (node.status === "retired") {
    return (
      <div className={sourceStyles.reviewPanel}>
        <h2>Review</h2>
        <p role="status">This node is already retired — no review action is available.</p>
      </div>
    );
  }

  return (
    <div className={sourceStyles.reviewPanel}>
      <h2>Review</h2>

      {error && (
        <p className={sourceStyles.fieldError} role="alert">
          {error}
        </p>
      )}

      {pendingAction === null && (
        <div className={sourceStyles.formActions}>
          {node.status === "draft" && (
            <button type="button" className={sourceStyles.button} disabled={submitting} onClick={() => setPendingAction("verify")}>
              Verify
            </button>
          )}
          <button type="button" className={sourceStyles.buttonDanger} disabled={submitting} onClick={() => setPendingAction("retire")}>
            Retire
          </button>
        </div>
      )}

      {pendingAction === "verify" && (
        <div className={sourceStyles.confirm} role="alertdialog" aria-labelledby="confirm-verify-heading">
          <p id="confirm-verify-heading">Verify &ldquo;{node.nodeCode}&rdquo;? This cannot be undone.</p>
          <div className={sourceStyles.formActions}>
            <button type="button" className={sourceStyles.button} disabled={submitting} onClick={() => onVerify(node.id)}>
              {submitting ? "Verifying…" : "Confirm verification"}
            </button>
            <button type="button" className={sourceStyles.buttonSecondary} disabled={submitting} onClick={() => setPendingAction(null)}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {pendingAction === "retire" && (
        <div className={sourceStyles.confirm} role="alertdialog" aria-labelledby="confirm-retire-heading">
          <p id="confirm-retire-heading">Retire &ldquo;{node.nodeCode}&rdquo;? This cannot be undone.</p>
          <div className={sourceStyles.formActions}>
            <button type="button" className={sourceStyles.buttonDanger} disabled={submitting} onClick={() => onRetire(node.id)}>
              {submitting ? "Retiring…" : "Confirm retirement"}
            </button>
            <button type="button" className={sourceStyles.buttonSecondary} disabled={submitting} onClick={() => setPendingAction(null)}>
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
