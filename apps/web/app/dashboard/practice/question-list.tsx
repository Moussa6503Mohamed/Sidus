"use client";

import { useState } from "react";
import { getOptionState } from "@/lib/design/option-state";
import { ApiError, createPracticeAttempt, submitPracticeAttempt } from "./api-client";
import styles from "./styles.module.css";
import type { LearnerAttemptResult, LearnerQuestion } from "./types";

interface QuestionListProps { questions: LearnerQuestion[] }

interface QuestionAttemptState {
  selectedOptionId?: string;
  attemptId?: string;
  submitting?: boolean;
  error?: string;
  result?: LearnerAttemptResult;
}

function messageFor(error: unknown): string {
  return error instanceof ApiError ? error.message : "Answer could not be submitted. Try again.";
}

export function QuestionList({ questions }: QuestionListProps) {
  const [states, setStates] = useState<Record<string, QuestionAttemptState>>({});

  function update(questionId: string, updateState: Partial<QuestionAttemptState>) {
    setStates((current) => ({ ...current, [questionId]: { ...current[questionId], ...updateState } }));
  }

  async function submit(question: LearnerQuestion) {
    const state = states[question.id] ?? {};
    if (!state.selectedOptionId || state.submitting || state.result) return;
    update(question.id, { submitting: true, error: undefined });
    try {
      const attemptId = state.attemptId ?? (await createPracticeAttempt(question.id)).attemptId;
      update(question.id, { attemptId });
      const result = await submitPracticeAttempt(attemptId, state.selectedOptionId);
      update(question.id, { result, submitting: false });
    } catch (error) {
      update(question.id, { submitting: false, error: messageFor(error) });
    }
  }

  return (
    <ul className={styles.list}>
      {questions.map((question) => {
        const state = states[question.id] ?? {};
        const result = state.result;
        const options = question.options ?? [];
        const optionLabels = new Map(options.map((option) => [option.id, option.label]));
        return (
          <li key={question.id} className={styles.card}>
            <p className={styles.prompt}>{question.prompt}</p>
            {question.responseType === "multiple_choice" && question.options ? (
              <>
                <div
                  className={styles.options}
                  role={result ? "list" : "radiogroup"}
                  aria-label={result ? "Options, marked" : "Answer options"}
                >
                  {options.map((option) => {
                    const optionState = getOptionState({
                      optionId: option.id,
                      selectedOptionId: state.selectedOptionId,
                      correctOptionId: result?.correctOptionId,
                      isMarked: Boolean(result),
                      disabled: Boolean(state.submitting),
                    });
                    return (
                      <button
                        key={option.id}
                        type="button"
                        className={styles.optionButton}
                        data-option-state={optionState.state}
                        role={result ? "listitem" : "radio"}
                        aria-checked={result ? undefined : state.selectedOptionId === option.id}
                        disabled={Boolean(state.submitting || result)}
                        onClick={() => update(question.id, { selectedOptionId: option.id, error: undefined })}
                      >
                        <span className={styles.optionKey} aria-hidden="true">
                          {option.label.charAt(0)}
                        </span>
                        <span>{option.label}</span>
                        {optionState.tag && <strong className={styles.optionTag}>{optionState.tag}</strong>}
                      </button>
                    );
                  })}
                </div>
                {!result && (
                  <button
                    type="button"
                    className={styles.button}
                    disabled={!state.selectedOptionId || state.submitting}
                    onClick={() => void submit(question)}
                  >
                    {state.submitting ? "Submitting…" : "Submit answer"}
                  </button>
                )}
                {state.error && (
                  <p className={styles.bannerError} role="alert">
                    {state.error}{" "}
                    <button type="button" className={styles.buttonSecondary} onClick={() => void submit(question)}>
                      Retry
                    </button>
                  </p>
                )}
                {result && (
                  <section className={styles.feedback} aria-label="Answer feedback" aria-live="polite">
                    <p className="sidus-visually-hidden">
                      You answered {optionLabels.get(result.selectedOptionId) ?? result.selectedOptionId}. The
                      correct answer is {optionLabels.get(result.correctOptionId) ?? result.correctOptionId}.{" "}
                      {result.awardedMarks} of {result.maxMarks} mark{result.maxMarks === 1 ? "" : "s"}.
                    </p>
                    <h2>{result.isCorrect ? "Correct" : "Incorrect"}</h2>
                    <p><strong>Score:</strong> {result.awardedMarks} / {result.maxMarks}</p>
                    <h3>Correct answer explanation</h3>
                    <p>{result.feedback.correctExplanation}</p>
                    <h3>Why other options are wrong</h3>
                    <ul>
                      {result.feedback.incorrectExplanations.map((item) => (
                        <li key={item.optionId}>
                          <strong>{optionLabels.get(item.optionId) ?? item.optionId}:</strong> {item.explanation}
                        </li>
                      ))}
                    </ul>
                  </section>
                )}
              </>
            ) : (
              <p className={styles.loading}>Practice submission is available for multiple-choice questions only.</p>
            )}
          </li>
        );
      })}
    </ul>
  );
}
