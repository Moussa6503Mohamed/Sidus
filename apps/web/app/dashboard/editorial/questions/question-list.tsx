import sourceStyles from "../sources/styles.module.css";
import styles from "./styles.module.css";
import type { CurriculumMapNode, Question } from "./types";

interface QuestionListProps {
  questions: Question[];
  nodes: CurriculumMapNode[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

export function QuestionList({ questions, nodes, selectedId, onSelect }: QuestionListProps) {
  const nodeNames = new Map(nodes.map((node) => [node.id, `${node.nodeCode} — ${node.label}`]));
  return (
    <div className={sourceStyles.tableWrap}>
      <table>
        <thead><tr><th>Question</th><th>Status</th><th>Response</th><th>Language</th><th>Node</th><th>Revision</th></tr></thead>
        <tbody>
          {questions.map((question, index) => (
            <tr key={question.id} className={selectedId === question.id ? sourceStyles.rowSelected : undefined}>
              <td><button type="button" className={sourceStyles.rowButton} onClick={() => onSelect(question.id)}>Question {index + 1}</button></td>
              <td><span className={styles.badge} data-status={question.status}>{question.status}</span></td>
              <td>{question.responseType.replaceAll("_", " ")}</td>
              <td>{question.language}</td>
              <td>{nodeNames.get(question.curriculumMapNodeId) ?? question.curriculumMapNodeId}</td>
              <td>{question.contentRevision}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
