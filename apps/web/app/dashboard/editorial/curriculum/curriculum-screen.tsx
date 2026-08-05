import { isEditorialRole } from "@/lib/editorial/permissions";
import type { EditorialRole } from "@/lib/editorial/permissions";
import { AccessDenied } from "./access-denied";
import { CurriculumWorkspace } from "./workspace";

interface CurriculumMapScreenProps {
  role: EditorialRole;
}

/** Decides AccessDenied vs. the interactive workspace from the caller's UI-visibility role.
 * This is display-only: every mutation the workspace triggers is still authorized by Core. */
export function CurriculumMapScreen({ role }: CurriculumMapScreenProps) {
  if (!isEditorialRole(role)) {
    return <AccessDenied />;
  }
  return <CurriculumWorkspace role={role} />;
}
