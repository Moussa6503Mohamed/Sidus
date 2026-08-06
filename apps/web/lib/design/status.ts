/**
 * Single source of truth for lifecycle-status presentation across the app: label, icon shape,
 * color tone, and border treatment. Colour is never the only signal — every consumer (the shared
 * StatusBadge component, MapTree-style indent dots, etc.) reads from this map instead of
 * duplicating status -> label/colour logic.
 */

export type LifecycleStatus =
  | "draft"
  | "pending"
  | "approved"
  | "verified"
  | "rejected"
  | "retired"
  | "expired";

export type StatusTone = "neutral" | "warning" | "success" | "danger" | "muted";
export type StatusBorderStyle = "dashed" | "solid" | "outline";
export type StatusIcon =
  | "pencil"
  | "clock"
  | "check-circle"
  | "shield-check"
  | "cross-circle"
  | "archive";

export interface StatusVisual {
  label: string;
  tone: StatusTone;
  borderStyle: StatusBorderStyle;
  icon: StatusIcon;
  /** True when the label should render with a strikethrough (retired/expired). */
  struck: boolean;
}

export const STATUS_VISUALS: Record<LifecycleStatus, StatusVisual> = {
  draft: { label: "Draft", tone: "neutral", borderStyle: "dashed", icon: "pencil", struck: false },
  pending: { label: "Pending review", tone: "warning", borderStyle: "solid", icon: "clock", struck: false },
  approved: { label: "Approved", tone: "success", borderStyle: "solid", icon: "check-circle", struck: false },
  verified: { label: "Verified", tone: "success", borderStyle: "solid", icon: "shield-check", struck: false },
  rejected: { label: "Rejected", tone: "danger", borderStyle: "solid", icon: "cross-circle", struck: false },
  retired: { label: "Retired", tone: "muted", borderStyle: "outline", icon: "archive", struck: true },
  expired: { label: "Expired", tone: "muted", borderStyle: "outline", icon: "archive", struck: true },
};

export function isLifecycleStatus(value: string): value is LifecycleStatus {
  return Object.hasOwn(STATUS_VISUALS, value);
}

export function statusVisual(status: LifecycleStatus): StatusVisual {
  return STATUS_VISUALS[status];
}
