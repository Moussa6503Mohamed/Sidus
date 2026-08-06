import styles from "./styles.module.css";

export function AccessDenied() {
  return (
    <div className={styles.accessDenied} role="status">
      <h1>No practice access</h1>
      <p>Your account does not have a recognized Sidus role. Ask an administrator for access.</p>
    </div>
  );
}
