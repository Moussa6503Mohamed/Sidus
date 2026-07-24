import type { Metadata } from "next";
import {
  ClerkProvider,
  Show,
  SignInButton,
  SignUpButton,
  UserButton,
} from "@clerk/nextjs";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Sidus Observatory",
  description: "Sidus academic preparation",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <ClerkProvider>
      <html lang="en">
        <body>
          <header
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              padding: "0.75rem 1rem",
              borderBottom: "1px solid #e5e7eb",
            }}
          >
            <Link href="/" style={{ fontWeight: 600 }}>
              Sidus Observatory
            </Link>
            <nav style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
              <Show when="signed-out">
                <SignInButton />
                <SignUpButton />
              </Show>
              <Show when="signed-in">
                <Link href="/dashboard">Dashboard</Link>
                <UserButton />
              </Show>
            </nav>
          </header>
          {children}
        </body>
      </html>
    </ClerkProvider>
  );
}
