import { auth } from "@clerk/nextjs/server";

// Protected placeholder. auth.protect() forces authentication (and dynamic rendering); the
// Clerk proxy also guards this route. No content-source data is exposed here yet.
export default async function DashboardPage() {
  const { userId } = await auth.protect();

  return (
    <main style={{ padding: "1.5rem" }}>
      <h1>Dashboard</h1>
      <p>Signed in as <code>{userId}</code>.</p>
      <p>Content tools will appear here as later tasks land.</p>
    </main>
  );
}
