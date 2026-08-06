"use client";

import { useState } from "react";
import styles from "./styles.module.css";
import type { LearnerQuestion } from "./types";

interface QuestionListProps {
  questions: LearnerQuestion[];
}

/**
 * Renders each question's prompt and, for multiple-choice questions, its options as selectable
 * buttons. Selecting an option only sets local highlight state — there is no submit, no marking,
 * no answer reveal, no timer, no attempt/session creation, and no AI call anywhere in this
 * component or its data. The projection itself structurally cannot carry an answer key (see
 * LearnerQuestion in packages/shared/src/contracts.ts), so there is nothing to reveal even if a
 * "reveal" affordance were added later.
 */
export function QuestionList({ questions }: QuestionListProps) {
  const [selected, setSelected] = useState<Record<string, string>>({});

  return (
    <ul className={styles.list}>
      {questions.map((question) => (
        <li key={question.id} className={styles.card}>
          <p className={styles.prompt}>{question.prompt}</p>
          {question.responseType === "multiple_choice" && question.options && (
            <div className={styles.options} role="group" aria-label="Answer options">
              {question.options.map((option) => (
                <button
                  key={option.id}
                  type="button"
                  className={styles.optionButton}
                  aria-pressed={selected[question.id] === option.id}
                  onClick={() =>
                    setSelected((prev) => ({
                      ...prev,
                      [question.id]: prev[question.id] === option.id ? "" : option.id,
                    }))
                  }
                >
                  {option.label}
                </button>
              ))}
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}
