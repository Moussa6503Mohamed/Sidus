import { requireEditorialRole } from "@/lib/editorial/role";
import { EditorialQuestionsScreen } from "./editorial-questions-screen";

export default async function EditorialQuestionsPage() {
  const role = await requireEditorialRole();
  return <main style={{ padding: "1.5rem" }}><EditorialQuestionsScreen role={role} /></main>;
}
