import { statusVisual, type LifecycleStatus } from "@/lib/design/status";
import { StatusGlyph } from "./icons";
import styles from "./StatusBadge.module.css";

interface StatusBadgeProps {
  status: LifecycleStatus;
}

/** The one place lifecycle status renders as icon + label + border. Every status surface
 * (content sources, curriculum-map nodes, questions, rubrics) is a client of this component so a
 * new state is added once, in lib/design/status.ts, not re-implemented per page. */
export function StatusBadge({ status }: StatusBadgeProps) {
  const visual = statusVisual(status);
  return (
    <span className={styles.badge} data-tone={visual.tone} data-border={visual.borderStyle}>
      <StatusGlyph icon={visual.icon} />
      <span className={visual.struck ? styles.struck : undefined}>{visual.label}</span>
    </span>
  );
}
