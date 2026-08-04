import { requireEditorialRole } from "@/lib/editorial/role";
import { EditorialSourcesScreen } from "./editorial-sources-screen";

export default async function EditorialSourcesPage() {
  const role = await requireEditorialRole();

  return (
    <main style={{ padding: "1.5rem" }}>
      <EditorialSourcesScreen role={role} />
    </main>
  );
}
