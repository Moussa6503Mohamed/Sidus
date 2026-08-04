"use client";

import { useState } from "react";
import { missingApprovalFields } from "./missing-fields";
import styles from "./styles.module.css";
import type { ContentSource } from "./types";

interface ReviewActionsProps {
  source: ContentSource;
  submitting: boolean;
  error: string | null;
  onApprove: (id: string) => void;
  onReject: (id: string, reason: string) => void;
}

type PendingAction = "approve" | "reject" | null;

export function ReviewActions({ source, submitting, error, onApprove, onReject }: ReviewActionsProps) {
  const [pendingAction, setPendingAction] = useState<PendingAction>(null);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | null>(null);

  if (source.status !== "pending") {
    return (
      <div className={styles.reviewPanel}>
        <h2>Review</h2>
        <p role="status">This source is already {source.status} — no review action is available.</p>
      </div>
    );
  }

  const missing = missingApprovalFields(source);

  function cancel() {
    setPendingAction(null);
    setReason("");
    setReasonError(null);
  }

  function confirmReject() {
    if (reason.trim() === "") {
      setReasonError("A reason is required to reject a source.");
      return;
    }
    onReject(source.id, reason.trim());
  }

  return (
    <div className={styles.reviewPanel}>
      <h2>Review</h2>

      {missing.length > 0 && (
        <p className={styles.bannerEmpty} role="status">
          Missing before this source can be approved: {missing.join(", ")}.
        </p>
      )}
      {error && (
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      )}

      {pendingAction === null && (
        <div className={styles.formActions}>
          <button
            type="button"
            className={styles.button}
            disabled={submitting || missing.length > 0}
            onClick={() => setPendingAction("approve")}
          >
            Approve
          </button>
          <button
            type="button"
            className={styles.buttonDanger}
            disabled={submitting}
            onClick={() => setPendingAction("reject")}
          >
            Reject
          </button>
        </div>
      )}

      {pendingAction === "approve" && (
        <div className={styles.confirm} role="alertdialog" aria-labelledby="confirm-approve-heading">
          <p id="confirm-approve-heading">Approve &ldquo;{source.title}&rdquo;? This cannot be undone.</p>
          <div className={styles.formActions}>
            <button
              type="button"
              className={styles.button}
              disabled={submitting}
              onClick={() => onApprove(source.id)}
            >
              {submitting ? "Approving…" : "Confirm approval"}
            </button>
            <button type="button" className={styles.buttonSecondary} disabled={submitting} onClick={cancel}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {pendingAction === "reject" && (
        <div className={styles.confirm} role="alertdialog" aria-labelledby="confirm-reject-heading">
          <p id="confirm-reject-heading">Reject &ldquo;{source.title}&rdquo;? This cannot be undone.</p>
          <div className={styles.field}>
            <label htmlFor="reject-reason">Reason (required)</label>
            <textarea
              id="reject-reason"
              value={reason}
              onChange={(e) => {
                setReason(e.target.value);
                setReasonError(null);
              }}
              rows={3}
              disabled={submitting}
              aria-describedby={reasonError ? "reject-reason-error" : undefined}
            />
            {reasonError && (
              <p id="reject-reason-error" className={styles.fieldError} role="alert">
                {reasonError}
              </p>
            )}
          </div>
          <div className={styles.formActions}>
            <button type="button" className={styles.buttonDanger} disabled={submitting} onClick={confirmReject}>
              {submitting ? "Rejecting…" : "Confirm rejection"}
            </button>
            <button type="button" className={styles.buttonSecondary} disabled={submitting} onClick={cancel}>
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
