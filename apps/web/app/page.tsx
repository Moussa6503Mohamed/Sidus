import { Show, SignInButton, SignUpButton } from "@clerk/nextjs";
import Link from "next/link";

export default function Home() {
  return (
    <main style={{ padding: "1.5rem" }}>
      <h1>Sidus Observatory</h1>
      <p>Biology vertical-slice foundation.</p>
      <Show when="signed-out">
        <p>Sign in or create an account to continue.</p>
        <div style={{ display: "flex", gap: "0.75rem" }}>
          <SignInButton />
          <SignUpButton />
        </div>
      </Show>
      <Show when="signed-in">
        <p>
          You are signed in. Go to your <Link href="/dashboard">dashboard</Link>.
        </p>
      </Show>
    </main>
  );
}
