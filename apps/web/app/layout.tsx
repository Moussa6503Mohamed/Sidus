import type { Metadata } from "next";
import {
  ClerkProvider,
  Show,
  SignInButton,
  SignUpButton,
  UserButton,
} from "@clerk/nextjs";
import Link from "next/link";
import { getOptionalEditorialRole } from "@/lib/editorial/role";
import { isEditorialRole } from "@/lib/editorial/permissions";
import { Logo } from "@/components/brand/Logo";
import { RoleChip } from "@/components/nav/RoleChip";
import { ThemeToggle } from "@/components/theme/ThemeToggle";
import { THEME_BOOTSTRAP_SCRIPT } from "@/components/theme/theme-script";
import navStyles from "@/components/nav/nav.module.css";
import "@/styles/tokens.css";

export const metadata: Metadata = {
  title: "Sidus Observatory",
  description: "Sidus academic preparation",
};

export default async function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const role = await getOptionalEditorialRole();
  const showEditorialNav = isEditorialRole(role);

  return (
    <ClerkProvider>
      <html lang="en" suppressHydrationWarning>
        {/* Constant script, no interpolated values — applies the persisted theme before first
            paint so there is no light/dark flash. See components/theme/theme-script.ts. */}
        <head>
          <script dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP_SCRIPT }} />
        </head>
        <body>
          <a href="#main" className="sidus-skip-link">
            Skip to main content
          </a>
          <header className={navStyles.topnav} role="banner">
            <Link href="/" className={navStyles.brandLink} aria-label="Sidus home">
              <Logo variant="lockup" size="sm" />
            </Link>
            <nav aria-label="Primary" className={navStyles.navlinks}>
              <Show when="signed-in">
                <Link href="/dashboard" className={navStyles.navlink}>
                  Dashboard
                </Link>
                {role !== "unknown" && (
                  <>
                    <Link href="/dashboard/practice" className={navStyles.navlink}>
                      Practice
                    </Link>
                    <Link href="/dashboard/exam" className={navStyles.navlink}>
                      Exam
                    </Link>
                  </>
                )}
                {showEditorialNav && (
                  <>
                    <span className={navStyles.navGroupLabel} aria-hidden="true">
                      Editorial
                    </span>
                    <Link href="/dashboard/editorial/sources" className={navStyles.navlink}>
                      Sources
                    </Link>
                    <Link href="/dashboard/editorial/curriculum" className={navStyles.navlink}>
                      Curriculum map
                    </Link>
                    <Link href="/dashboard/editorial/questions" className={navStyles.navlink}>
                      Questions
                    </Link>
                  </>
                )}
              </Show>
            </nav>
            <div className={navStyles.spacer} />
            <ThemeToggle />
            <Show when="signed-out">
              <SignInButton />
              <SignUpButton />
            </Show>
            <Show when="signed-in">
              {role !== "unknown" && <RoleChip role={role} />}
              <UserButton />
            </Show>
          </header>
          {children}
        </body>
      </html>
    </ClerkProvider>
  );
}
