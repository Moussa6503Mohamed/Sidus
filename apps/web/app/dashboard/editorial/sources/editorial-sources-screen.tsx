import { isEditorialRole } from "@/lib/editorial/permissions";
import type { EditorialRole } from "@/lib/editorial/permissions";
import { AccessDenied } from "./access-denied";
import { EditorialSourcesWorkspace } from "./workspace";

interface EditorialSourcesScreenProps {
  role: EditorialRole;
}

/** Decides AccessDenied vs. the interactive workspace from the caller's UI-visibility role.
 * This is display-only: every mutation the workspace triggers is still authorized by Core. */
export function EditorialSourcesScreen({ role }: EditorialSourcesScreenProps) {
  if (!isEditorialRole(role)) {
    return <AccessDenied />;
  }
  return <EditorialSourcesWorkspace role={role} />;
}
