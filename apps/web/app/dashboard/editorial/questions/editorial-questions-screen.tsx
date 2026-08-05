import { isEditorialRole, type EditorialRole } from "@/lib/editorial/permissions";
import { AccessDenied } from "./access-denied";
import { QuestionsWorkspace } from "./workspace";

export function EditorialQuestionsScreen({ role }: { role: EditorialRole }) {
  if (!isEditorialRole(role)) return <AccessDenied />;
  return <QuestionsWorkspace role={role} />;
}
