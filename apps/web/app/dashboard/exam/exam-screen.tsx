import { canAccessPractice } from "@/lib/learner/permissions";
import type { EditorialRole } from "@/lib/editorial/permissions";
import { AccessDenied } from "../practice/access-denied";
import { ExamWorkspace } from "./workspace";

export function ExamScreen({ role }: { role: EditorialRole }) {
  return canAccessPractice(role) ? <ExamWorkspace /> : <AccessDenied />;
}
