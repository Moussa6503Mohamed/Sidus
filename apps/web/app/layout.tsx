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
import { cookies } from "next/headers";
import { SwRegistry } from "@/components/sw-registry";
import { LocaleToggle } from "@/components/nav/LocaleToggle";

export const metadata: Metadata = {
  title: "Sidus Observatory",
  description: "Sidus academic preparation",
};

export default async function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const role = await getOptionalEditorialRole();
  const showEditorialNav = isEditorialRole(role);
  
  const cookieStore = await cookies();
  const currentLocale = cookieStore.get("sidus_locale")?.value || "en";
  const dir = currentLocale === "ar" ? "rtl" : "ltr";

  return (
    <ClerkProvider>
      <html lang={currentLocale} dir={dir} suppressHydrationWarning>
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
                    <Link href="/dashboard/assignments" className={navStyles.navlink}>Assignments</Link>
                  </>
                )}
                {(role === "teacher" || role === "admin") && <Link href="/dashboard/teacher" className={navStyles.navlink}>Classes</Link>}
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
                    {role === "admin" && <Link href="/dashboard/editorial/uploads" className={navStyles.navlink}>Private intake</Link>}
                  </>
                )}
              </Show>
            </nav>
            <div className={navStyles.spacer} />
            <LocaleToggle currentLocale={currentLocale} />
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
          <SwRegistry />
        </body>
      </html>
    </ClerkProvider>
  );
}
