import { Show, SignInButton, SignUpButton } from "@clerk/nextjs";
import Link from "next/link";
import { Button } from "@/components/ui/Button";
import styles from "./page.module.css";

export default function Home() {
  return (
    <main id="main" className={styles.hero}>
      <span className={styles.eyebrow}>Sidus Observatory</span>
      <h1>Prepare with precision</h1>
      <p className={styles.lede}>Biology vertical-slice foundation.</p>
      <Show when="signed-out">
        <p className={styles.lede}>Sign in or create an account to continue.</p>
        <div className={styles.actions}>
          <SignInButton>
            <Button variant="primary">Sign in</Button>
          </SignInButton>
          <SignUpButton>
            <Button variant="secondary">Create account</Button>
          </SignUpButton>
        </div>
      </Show>
      <Show when="signed-in">
        <p className={styles.signedIn}>
          You are signed in. Go to your <Link href="/dashboard">dashboard</Link>.
        </p>
      </Show>
    </main>
  );
}
