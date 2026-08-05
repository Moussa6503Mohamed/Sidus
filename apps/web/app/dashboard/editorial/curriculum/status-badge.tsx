import styles from "./styles.module.css";
import type { CurriculumMapNodeStatus } from "./types";

const LABELS: Record<CurriculumMapNodeStatus, string> = {
  draft: "Draft",
  verified: "Verified",
  retired: "Retired",
};

export function StatusBadge({ status }: { status: CurriculumMapNodeStatus }) {
  return (
    <span className={styles.badge} data-status={status}>
      {LABELS[status]}
    </span>
  );
}
