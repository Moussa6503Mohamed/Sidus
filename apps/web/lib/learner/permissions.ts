import type { EditorialRole } from "@/lib/editorial/permissions";

/**
 * UI-visibility check for the practice workspace: every recognized Sidus role (learner, editor,
 * reviewer, admin) may use it — only an unrecognized/missing role is denied. This mirrors Core's
 * `learner_question:read` permission (held by every role in the auth matrix); Core remains the
 * sole authorization authority (D-0006) — a stale or wrong value here can only hide the workspace
 * Core would have allowed, never show one Core would refuse.
 *
 * Reuses `EditorialRole`/session-claim parsing from lib/editorial/role.ts and permissions.ts:
 * those are a generic verified-Clerk-claim reader, not editorial business logic, so sharing them
 * does not couple this read-only learner surface to the editorial BFF/route/permission surface.
 */
export function canAccessPractice(role: EditorialRole): boolean {
  return role !== "unknown";
}
