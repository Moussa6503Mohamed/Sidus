import { canAccessPractice } from "@/lib/learner/permissions";
import type { EditorialRole } from "@/lib/editorial/permissions";
import { AccessDenied } from "./access-denied";
import { PracticeWorkspace } from "./workspace";

interface PracticeScreenProps {
  role: EditorialRole;
}

/** Decides AccessDenied vs. the interactive workspace from the caller's UI-visibility role.
 * This is display-only: Core's learner_question:read permission (held by every recognized role)
 * is the sole authorization authority for every read the workspace triggers. */
export function PracticeScreen({ role }: PracticeScreenProps) {
  if (!canAccessPractice(role)) {
    return <AccessDenied />;
  }
  return <PracticeWorkspace />;
}
