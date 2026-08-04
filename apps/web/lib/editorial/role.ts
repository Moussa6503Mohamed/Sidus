import "server-only";
import { auth } from "@clerk/nextjs/server";
import { SIDUS_ROLE_CLAIM } from "@sidus/shared";
import { type EditorialRole, parseSidusRole } from "./permissions";

/** Reads the verified `sidus_role` session claim (populated by Clerk from the signed token,
 * never client-suppliable). UI-visibility only — see EditorialRole. */
function roleFromSessionClaims(sessionClaims: unknown): EditorialRole {
  const claims = sessionClaims as Record<string, unknown> | null | undefined;
  return parseSidusRole(claims?.[SIDUS_ROLE_CLAIM]);
}

/** For layout-level nav gating on routes that must stay reachable while signed out (home,
 * sign-in, sign-up): never redirects. A signed-out visitor resolves to "unknown". */
export async function getOptionalEditorialRole(): Promise<EditorialRole> {
  const { sessionClaims } = await auth();
  return roleFromSessionClaims(sessionClaims);
}

/** For a protected page: requires a signed-in session (redirects to sign-in otherwise, same
 * as every other `/dashboard` route) and returns the caller's UI-visibility role. */
export async function requireEditorialRole(): Promise<EditorialRole> {
  const { sessionClaims } = await auth.protect();
  return roleFromSessionClaims(sessionClaims);
}
