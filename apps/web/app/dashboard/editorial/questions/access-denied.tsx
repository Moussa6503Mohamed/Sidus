import sourceStyles from "../sources/styles.module.css";

export function AccessDenied() {
  return <div className={sourceStyles.accessDenied} role="status">
    <h1>No editorial access</h1>
    <p>Your account cannot access question and rubric workflow. Editor, reviewer, or admin role is required.</p>
  </div>;
}
