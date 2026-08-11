"use client";

import { useRef, useState, type KeyboardEvent } from "react";
import { getOptionState } from "@/lib/design/option-state";
import { ApiError, createPracticeAttempt, submitPracticeAttempt } from "./api-client";
import styles from "./styles.module.css";
import type { LearnerAnswer, LearnerAttemptResult, LearnerQuestion, LearnerQuestionOption } from "./types";

interface QuestionListProps { questions: LearnerQuestion[] }

interface QuestionAttemptState {
  selectedOptionId?: string;
  selectedOptionIds?: string[];
  text?: string;
  value?: string;
  attemptId?: string;
  submitting?: boolean;
  error?: string;
  result?: LearnerAttemptResult;
}

function messageFor(error: unknown): string {
  return error instanceof ApiError ? error.message : "Answer could not be submitted. Try again.";
}

const NEXT_KEYS = new Set(["ArrowRight", "ArrowDown"]);
const PREV_KEYS = new Set(["ArrowLeft", "ArrowUp"]);
const SELECT_KEYS = new Set([" ", "Spacebar", "Enter"]);

interface OptionGroupProps {
  questionId: string;
  options: LearnerQuestionOption[];
  state: QuestionAttemptState;
  onSelect: (optionId: string) => void;
}

/**
 * ARIA APG radiogroup: one roving tab stop (selected option, else the first option), arrow keys
 * move focus AND selection together, Space/Enter select the already-focused option. After
 * marking the group becomes a plain read-only list — see D-0019 "Update (T-0017 P1 review fix)".
 */
function OptionGroup({ questionId, options, state, onSelect }: OptionGroupProps) {
  const buttonRefs = useRef(new Map<string, HTMLButtonElement>());
  const result = state.result;
  const isInteractive = !result && !state.submitting;
  const rovingTargetId = state.selectedOptionId ?? options[0]?.id;

  function focusOption(optionId: string) {
    buttonRefs.current.get(optionId)?.focus();
  }

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (!isInteractive) return;
    const count = options.length;
    if (NEXT_KEYS.has(event.key)) {
      event.preventDefault();
      const next = options[(index + 1) % count];
      onSelect(next.id);
      focusOption(next.id);
    } else if (PREV_KEYS.has(event.key)) {
      event.preventDefault();
      const prev = options[(index - 1 + count) % count];
      onSelect(prev.id);
      focusOption(prev.id);
    } else if (event.key === "Home") {
      event.preventDefault();
      onSelect(options[0].id);
      focusOption(options[0].id);
    } else if (event.key === "End") {
      event.preventDefault();
      const last = options[count - 1];
      onSelect(last.id);
      focusOption(last.id);
    } else if (SELECT_KEYS.has(event.key)) {
      event.preventDefault();
      onSelect(options[index].id);
    }
  }

  return (
    <div
      className={styles.options}
      role={result ? "list" : "radiogroup"}
      aria-labelledby={result ? undefined : `question-prompt-${questionId}`}
      aria-label={result ? "Options, marked" : undefined}
    >
      {options.map((option, index) => {
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
            ref={(node) => {
              if (node) buttonRefs.current.set(option.id, node);
              else buttonRefs.current.delete(option.id);
            }}
            type="button"
            className={styles.optionButton}
            data-option-state={optionState.state}
            role={result ? "listitem" : "radio"}
            aria-checked={result ? undefined : state.selectedOptionId === option.id}
            tabIndex={result ? undefined : option.id === rovingTargetId ? 0 : -1}
            disabled={Boolean(state.submitting || result)}
            onClick={() => onSelect(option.id)}
            onKeyDown={(event) => handleKeyDown(event, index)}
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
  );
}

export function QuestionList({ questions }: QuestionListProps) {
  const [states, setStates] = useState<Record<string, QuestionAttemptState>>({});

  function update(questionId: string, updateState: Partial<QuestionAttemptState>) {
    setStates((current) => ({ ...current, [questionId]: { ...current[questionId], ...updateState } }));
  }

  function answerFor(question: LearnerQuestion, state: QuestionAttemptState): LearnerAnswer | null {
    if (question.responseType === "multiple_choice" && state.selectedOptionId) return { selectedOptionId: state.selectedOptionId };
    if (question.responseType === "multi_select" && state.selectedOptionIds?.length) return { selectedOptionIds: state.selectedOptionIds };
    if ((question.responseType === "exact_short_answer" || question.responseType === "short_answer" || question.responseType === "structured_response" || question.responseType === "essay") && state.text?.trim()) return { text: state.text.trim() };
    if (question.responseType === "numeric_answer" && state.value?.trim() && Number.isFinite(Number(state.value))) return { value: Number(state.value) };
    return null;
  }
  async function submit(question: LearnerQuestion) {
    const state = states[question.id] ?? {};
    const answer = answerFor(question, state); if (!answer || state.submitting || state.result) return;
    update(question.id, { submitting: true, error: undefined });
    try {
      const attemptId = state.attemptId ?? (await createPracticeAttempt(question.id)).attemptId;
      update(question.id, { attemptId });
      const result = await submitPracticeAttempt(attemptId, answer);
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
            <p className={styles.prompt} id={`question-prompt-${question.id}`}>{question.prompt}</p>
            {question.responseType === "multiple_choice" && question.options ? (
              <>
                <OptionGroup
                  questionId={question.id}
                  options={options}
                  state={state}
                  onSelect={(optionId) => update(question.id, { selectedOptionId: optionId, error: undefined })}
                />
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
            ) : <>
              {question.responseType === "multi_select" && <fieldset className={styles.checkGroup}><legend>Select every answer that applies</legend>{options.map((option) => <label key={option.id} className={styles.checkOption}><input type="checkbox" disabled={Boolean(result || state.submitting)} checked={state.selectedOptionIds?.includes(option.id) ?? false} onChange={(event) => update(question.id, { selectedOptionIds: event.target.checked ? [...(state.selectedOptionIds ?? []), option.id] : (state.selectedOptionIds ?? []).filter((id) => id !== option.id), error: undefined })} /><span>{option.label}</span></label>)}</fieldset>}
              {question.responseType === "numeric_answer" && <label className={styles.answerField}>Numeric answer<input inputMode="decimal" value={state.value ?? ""} disabled={Boolean(result || state.submitting)} onChange={(event) => update(question.id, { value: event.target.value, error: undefined })} /></label>}
              {["exact_short_answer", "short_answer", "structured_response", "essay"].includes(question.responseType) && <label className={styles.answerField}>{question.responseType === "exact_short_answer" ? "Answer" : "Written response"}<textarea rows={question.responseType === "essay" ? 8 : 4} value={state.text ?? ""} disabled={Boolean(result || state.submitting)} onChange={(event) => update(question.id, { text: event.target.value, error: undefined })} /></label>}
              {!result && <button type="button" className={styles.button} disabled={!answerFor(question, state) || state.submitting} onClick={() => void submit(question)}>{state.submitting ? "Submitting…" : "Submit answer"}</button>}
              {state.error && <p className={styles.bannerError} role="alert">{state.error}</p>}
              {result && <section className={styles.feedback} aria-live="polite"><h2>{result.resultStatus === "pending_review" ? "Submitted for review" : result.isCorrect ? "Correct" : "Incorrect"}</h2>{result.resultStatus === "pending_review" ? <p>Your written response is saved. Marking is pending review.</p> : <p><strong>Score:</strong> {result.awardedMarks} / {result.maxMarks}</p>}</section>}
            </>}
          </li>
        );
      })}
    </ul>
  );
}
