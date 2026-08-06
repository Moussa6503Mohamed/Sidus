/**
 * Pure derivation of an MCQ option's visual/accessible state. Keeps correctness rendering out of
 * JSX and unit-testable — every option row (Practice Mode today, any future editorial preview)
 * reads from this instead of re-deriving selected/correct logic inline.
 */

export type MCQOptionState =
  | "default"
  | "selected"
  | "correct"
  | "incorrect"
  | "selected-incorrect"
  | "disabled";

export interface OptionStateResult {
  state: MCQOptionState;
  /** Explicit text tag shown alongside the option — colour is never the only signal. `null` for the plain default state. */
  tag: string | null;
}

export interface OptionStateInput {
  optionId: string;
  selectedOptionId?: string | null;
  /** Present only once the attempt is marked. */
  correctOptionId?: string | null;
  /** True once a result exists and options are read-only. */
  isMarked: boolean;
  /** True while a submission is in flight, before marking. */
  disabled?: boolean;
}

export function getOptionState({
  optionId,
  selectedOptionId,
  correctOptionId,
  isMarked,
  disabled = false,
}: OptionStateInput): OptionStateResult {
  const isSelected = selectedOptionId === optionId;

  if (!isMarked) {
    if (disabled) return { state: "disabled", tag: "Locked" };
    if (isSelected) return { state: "selected", tag: "Selected" };
    return { state: "default", tag: null };
  }

  const isCorrect = correctOptionId === optionId;
  if (isCorrect) {
    return { state: "correct", tag: isSelected ? "Your answer · correct" : "Correct answer" };
  }
  if (isSelected) {
    return { state: "selected-incorrect", tag: "Your answer · incorrect" };
  }
  return { state: "incorrect", tag: "Not correct" };
}
