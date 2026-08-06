import { StatusBadge as SharedStatusBadge } from "@/components/ui/StatusBadge";
import type { ContentSourceStatus } from "./types";

/** Thin, page-scoped wrapper so source-list.tsx/source-form.tsx keep importing from "./status-badge" —
 * the actual label/icon/tone/border logic lives once in lib/design/status.ts. */
export function StatusBadge({ status }: { status: ContentSourceStatus }) {
  return <SharedStatusBadge status={status} />;
}
