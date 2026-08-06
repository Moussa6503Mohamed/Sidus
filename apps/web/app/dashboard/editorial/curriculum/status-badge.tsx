import { StatusBadge as SharedStatusBadge } from "@/components/ui/StatusBadge";
import type { CurriculumMapNodeStatus } from "./types";

/** Thin, page-scoped wrapper so node-list.tsx/node-form.tsx keep importing from "./status-badge" —
 * the actual label/icon/tone/border logic lives once in lib/design/status.ts. */
export function StatusBadge({ status }: { status: CurriculumMapNodeStatus }) {
  return <SharedStatusBadge status={status} />;
}
