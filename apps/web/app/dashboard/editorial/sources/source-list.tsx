import { missingApprovalFields } from "./missing-fields";
import { StatusBadge } from "./status-badge";
import styles from "./styles.module.css";
import { visuallyHidden } from "./visually-hidden";
import type { ContentSource } from "./types";

interface SourceListProps {
  sources: ContentSource[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

export function SourceList({ sources, selectedId, onSelect }: SourceListProps) {
  return (
    <div className={styles.tableWrap}>
      <table>
        <caption style={visuallyHidden}>Content sources</caption>
        <thead>
          <tr>
            <th scope="col">Title</th>
            <th scope="col">Status</th>
            <th scope="col">Syllabus</th>
            <th scope="col">Catalogue link</th>
            <th scope="col">Missing for approval</th>
          </tr>
        </thead>
        <tbody>
          {sources.map((source) => {
            const missing = missingApprovalFields(source);
            const selected = source.id === selectedId;
            return (
              <tr key={source.id} className={selected ? styles.rowSelected : undefined}>
                <td>
                  <button
                    type="button"
                    className={styles.rowButton}
                    aria-current={selected ? "true" : undefined}
                    onClick={() => onSelect(source.id)}
                  >
                    {source.title || "(untitled)"}
                  </button>
                </td>
                <td>
                  <StatusBadge status={source.status} />
                </td>
                <td>{source.syllabusCode ?? "—"}</td>
                <td>{source.catalogueSyllabusId ? "Linked" : "Unlinked"}</td>
                <td className={styles.missing}>
                  {source.status === "approved" || missing.length === 0
                    ? "None"
                    : missing.join(", ")}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
