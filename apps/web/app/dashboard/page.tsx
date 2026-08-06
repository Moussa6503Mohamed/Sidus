import { auth } from "@clerk/nextjs/server";
import styles from "./dashboard.module.css";

// Protected placeholder. auth.protect() forces authentication (and dynamic rendering); the
// Clerk proxy also guards this route. No content-source data is exposed here yet.
export default async function DashboardPage() {
  const { userId } = await auth.protect();

  return (
    <main id="main" className={styles.page}>
      <h1>Dashboard</h1>
      <p>
        Signed in as <code className={styles.mono}>{userId}</code>.
      </p>
      <p className={styles.muted}>Content tools will appear here as later tasks land.</p>
    </main>
  );
}
