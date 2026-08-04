import { SIDUS_ROLES, type SidusRole } from "@sidus/shared";

/**
 * The web-side mirror of Core's deny-by-default role parsing (services/core/internal/auth
 * `ParseRole`): a missing or unrecognized `sidus_role` claim value is "unknown", never
 * silently treated as any known role. This is UI-only — Core is still the sole authorization
 * authority for every mutation (D-0006); a stale or incorrect value here only changes which
 * controls render, never what Core will accept.
 */
export type EditorialRole = SidusRole | "unknown";

/** editor, reviewer, and admin may read/create/update pending sources. */
export type EditingRole = Extract<SidusRole, "editor" | "reviewer" | "admin">;
/** Only reviewer and admin may approve/reject. */
export type ReviewingRole = Extract<SidusRole, "reviewer" | "admin">;

const KNOWN_ROLES: ReadonlySet<string> = new Set(SIDUS_ROLES);

export function parseSidusRole(raw: unknown): EditorialRole {
  if (typeof raw !== "string") {
    return "unknown";
  }
  const trimmed = raw.trim();
  return KNOWN_ROLES.has(trimmed) ? (trimmed as SidusRole) : "unknown";
}

export function isEditorialRole(role: EditorialRole): role is EditingRole {
  return role === "editor" || role === "reviewer" || role === "admin";
}

export function canReview(role: EditorialRole): role is ReviewingRole {
  return role === "reviewer" || role === "admin";
}
