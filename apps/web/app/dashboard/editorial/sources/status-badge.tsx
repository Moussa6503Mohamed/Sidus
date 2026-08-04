import styles from "./styles.module.css";
import type { ContentSourceStatus } from "./types";

const LABELS: Record<ContentSourceStatus, string> = {
  pending: "Pending",
  approved: "Approved",
  rejected: "Rejected",
  expired: "Expired",
};

export function StatusBadge({ status }: { status: ContentSourceStatus }) {
  return (
    <span className={styles.badge} data-status={status}>
      {LABELS[status]}
    </span>
  );
}
