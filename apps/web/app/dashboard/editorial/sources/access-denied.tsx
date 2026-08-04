import styles from "./styles.module.css";

export function AccessDenied() {
  return (
    <div className={styles.accessDenied} role="status">
      <h1>No editorial access</h1>
      <p>
        Your account does not have permission to view the source workflow. Ask an administrator
        to grant an editor, reviewer, or admin role if you believe this is a mistake.
      </p>
    </div>
  );
}
