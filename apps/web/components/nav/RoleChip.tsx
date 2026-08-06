import styles from "./nav.module.css";

interface RoleChipProps {
  role: string;
}

/** Role is always displayed, never implied. Presentation only — the server remains the sole
 * authorization authority; this chip cannot grant or hide anything by itself. */
export function RoleChip({ role }: RoleChipProps) {
  return <span className={styles.rolechip}>{role}</span>;
}
