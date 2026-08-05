import sourceStyles from "../sources/styles.module.css";
import { visuallyHidden } from "../sources/visually-hidden";
import styles from "./styles.module.css";
import { nodeDepths, parentCodeOf } from "./hierarchy";
import { StatusBadge } from "./status-badge";
import type { ContentSource, CurriculumMapNode } from "./types";

interface NodeListProps {
  nodes: CurriculumMapNode[];
  contentSources: ContentSource[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

const KIND_LABELS: Record<CurriculumMapNode["nodeKind"], string> = {
  topic: "Topic",
  objective: "Objective",
  practical_skill: "Practical skill",
  assessment_rule: "Assessment rule",
};

export function NodeList({ nodes, contentSources, selectedId, onSelect }: NodeListProps) {
  const depths = nodeDepths(nodes);
  const sourceTitle = (id: string) => contentSources.find((s) => s.id === id)?.title ?? id;

  return (
    <div className={sourceStyles.tableWrap}>
      <table>
        <caption style={visuallyHidden}>Curriculum-map nodes</caption>
        <thead>
          <tr>
            <th scope="col">Code</th>
            <th scope="col">Kind</th>
            <th scope="col">Label</th>
            <th scope="col">Parent</th>
            <th scope="col">Status</th>
            <th scope="col">Source</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((node) => {
            const selected = node.id === selectedId;
            const parentCode = parentCodeOf(node, nodes);
            return (
              <tr key={node.id} className={selected ? sourceStyles.rowSelected : undefined}>
                <td>
                  <span className={styles.indent} style={{ paddingLeft: `${(depths.get(node.id) ?? 0) * 1.25}rem` }}>
                    <button
                      type="button"
                      className={sourceStyles.rowButton}
                      aria-current={selected ? "true" : undefined}
                      onClick={() => onSelect(node.id)}
                    >
                      {node.nodeCode}
                    </button>
                  </span>
                </td>
                <td>{KIND_LABELS[node.nodeKind]}</td>
                <td>{node.label || "(untitled)"}</td>
                <td className={styles.hierarchyLabel}>{parentCode ?? "— root —"}</td>
                <td>
                  <StatusBadge status={node.status} />
                </td>
                <td>{sourceTitle(node.contentSourceId)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
