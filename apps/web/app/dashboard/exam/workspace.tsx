"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import {
  ApiError,
  createExamAttempt,
  listExamModules,
  listExamQuestions,
  listExamSyllabuses,
  submitExamAttempt,
} from "./api-client";
import { finalizeExam, type ExamRuntime } from "./finalization";
import styles from "../practice/styles.module.css";
import type { LearnerModule, LearnerQuestion, LearnerSyllabus } from "./types";

type Screen = "setup" | "taking" | "confirm" | "submitting" | "retry" | "results";
const DURATION_SECONDS = 30 * 60;

function errorMessage(error: unknown) {
  return error instanceof ApiError ? error.message : "Exam could not be completed. Retry remaining answers.";
}

function label(syllabus: LearnerSyllabus) {
  return syllabus.track ? `${syllabus.displayName} (${syllabus.track})` : syllabus.displayName;
}
function moduleLabel(module: LearnerModule) {
  return module.label ? `${module.code} — ${module.label}` : module.code;
}

function clock(seconds: number) {
  return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

export function ExamWorkspace({ durationSeconds = DURATION_SECONDS }: { durationSeconds?: number }) {
  const [syllabuses, setSyllabuses] = useState<LearnerSyllabus[] | null>(null);
  const [loadError, setLoadError] = useState<string>();
  const [syllabusId, setSyllabusId] = useState("");
  const [modules, setModules] = useState<LearnerModule[] | null>(null);
  const [moduleError, setModuleError] = useState<string>();
  const [moduleId, setModuleId] = useState("");
  const [count, setCount] = useState("all");
  const [questions, setQuestions] = useState<LearnerQuestion[]>([]);
  const [setupMessage, setSetupMessage] = useState<string>();
  const [screen, setScreen] = useState<Screen>("setup");
  const [index, setIndex] = useState(0);
  const [answers, setAnswers] = useState<Record<string, string | undefined>>({});
  const [flags, setFlags] = useState<Record<string, boolean>>({});
  const [remaining, setRemaining] = useState(DURATION_SECONDS);
  const [progress, setProgress] = useState("");
  const [finalError, setFinalError] = useState<string>();
  const runtime = useRef<ExamRuntime>({ attempts: {}, results: {} });
  const deadline = useRef<number | undefined>(undefined);
  const answersRef = useRef<Record<string, string | undefined>>({});
  const finalizing = useRef(false);
  const optionRefs = useRef(new Map<string, HTMLButtonElement>());
  const submitTrigger = useRef<HTMLButtonElement>(null);
  const confirmButton = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLElement>(null);

  async function loadSyllabuses() {
    setLoadError(undefined);
    try {
      setSyllabuses((await listExamSyllabuses()).items);
    } catch (error) {
      setLoadError(errorMessage(error));
    }
  }

  useEffect(() => {
    void loadSyllabuses();
  }, []);

  async function chooseSyllabus(next: string) {
    setSyllabusId(next);
    setModuleId("");
    setCount("all");
    setSetupMessage(undefined);
    setModuleError(undefined);
    if (!next) { setModules(null); return; }
    setModules(null);
    try { setModules((await listExamModules(next)).items); }
    catch (error) { setModules([]); setModuleError(errorMessage(error)); }
  }

  useEffect(() => {
    if ((screen !== "taking" && screen !== "confirm") || !deadline.current) return;
    const timer = window.setInterval(() => {
      const next = Math.max(0, Math.ceil((deadline.current! - Date.now()) / 1000));
      setRemaining(next);
      if (next === 0 && !finalizing.current) {
        window.clearInterval(timer);
        void submitAll(true);
      }
    }, 1000);
    return () => window.clearInterval(timer);
    // The timer intentionally uses only lifecycle state. Latest answers live in answersRef.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen]);

  useEffect(() => {
    if (screen !== "confirm") return;
    confirmButton.current?.focus();
    function trapFocus(event: globalThis.KeyboardEvent) {
      if (event.key !== "Tab" || !dialog.current) return;
      const focusable = Array.from(
        dialog.current.querySelectorAll<HTMLElement>(
          "button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled])",
        ),
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", trapFocus);
    return () => document.removeEventListener("keydown", trapFocus);
  }, [screen]);

  async function start(event: React.FormEvent) {
    event.preventDefault();
    setSetupMessage(undefined);
    try {
      const available = (await listExamQuestions(syllabusId, moduleId || undefined)).items.filter((question) => question.responseType === "multiple_choice" && Boolean(question.options?.length));
      const requested = count === "all" ? available.length : Number(count);
      if (!Number.isSafeInteger(requested) || requested < 1 || requested > available.length) {
        setSetupMessage(`Choose a whole number from 1 to ${available.length}, or All available questions.`);
        return;
      }
      setQuestions(available.slice(0, requested));
      answersRef.current = {};
      finalizing.current = false;
      setAnswers({});
      setFlags({});
      setIndex(0);
      setRemaining(durationSeconds);
      runtime.current = { attempts: {}, results: {} };
      deadline.current = Date.now() + durationSeconds * 1000;
      setScreen("taking");
    } catch (error) {
      setSetupMessage(errorMessage(error));
    }
  }

  async function submitAll(expired = false) {
    if (finalizing.current) return;
    finalizing.current = true;
    setScreen("submitting");
    setFinalError(undefined);
    try {
      await finalizeExam(questions, answersRef.current, runtime.current, {
        createAttempt: createExamAttempt,
        submitAttempt: submitExamAttempt,
      }, (done, total) => setProgress(`Marking ${done} of ${total} answered question${total === 1 ? "" : "s"}…`));
      setScreen("results");
    } catch (error) {
      finalizing.current = false;
      setFinalError(`${expired ? "Time ended. " : ""}${errorMessage(error)}`);
      setScreen("retry");
    }
  }

  if (syllabuses === null) {
    return <div className={styles.page}><h1>Exam Mode</h1>{loadError ? <p role="alert" className={styles.bannerError}>{loadError} <button className={styles.buttonSecondary} onClick={() => void loadSyllabuses()}>Retry</button></p> : <p role="status">Loading syllabuses…</p>}</div>;
  }
  if (screen === "setup") {
    return <div className={styles.page}><header className={styles.header}><h1>Exam Mode</h1><p>Local MVP. 30-minute countdown runs in this browser only; refreshing loses exam progress.</p></header><form className={styles.form} onSubmit={start}><div className={styles.field}><label htmlFor="exam-syllabus">Syllabus</label><select id="exam-syllabus" value={syllabusId} onChange={(event) => void chooseSyllabus(event.target.value)} required><option value="" disabled>Select a syllabus</option>{syllabuses.map((syllabus) => <option key={syllabus.id} value={syllabus.id}>{label(syllabus)}</option>)}</select></div>{syllabusId && <div className={styles.field}><label htmlFor="exam-module">Module</label><select id="exam-module" value={moduleId} onChange={(event) => setModuleId(event.target.value)} disabled={modules === null || Boolean(moduleError)}><option value="">All modules</option>{modules?.map((module) => <option key={module.id} value={module.id}>{moduleLabel(module)}</option>)}</select>{modules === null && !moduleError && <span role="status">Loading modules…</span>}{moduleError && <span role="alert">{moduleError} <button type="button" className={styles.buttonSecondary} onClick={() => void chooseSyllabus(syllabusId)}>Retry modules</button></span>}{modules?.length === 0 && !moduleError && <span>No modules with eligible questions yet.</span>}</div>}<div className={styles.field}><label htmlFor="exam-count">Questions</label><select id="exam-count" value={count === "all" ? "all" : "custom"} onChange={(event) => setCount(event.target.value === "all" ? "all" : count === "all" ? "1" : count)}><option value="all">All available</option><option value="custom">Choose a number</option></select>{count !== "all" && <input aria-label="Question count" type="number" min="1" step="1" value={count} onChange={(event) => setCount(event.target.value)} />}</div><button className={styles.button} disabled={!syllabusId || modules === null || Boolean(moduleError)}>Start exam</button></form>{setupMessage && <p role="alert" className={styles.bannerError}>{setupMessage}</p>}</div>;
  }
  if (screen === "submitting" || screen === "retry") {
    return <div className={styles.page}><h1>Finalising exam</h1><p role="status">{progress || "Preparing answers…"}</p>{screen === "retry" && <p role="alert" className={styles.bannerError}>{finalError} <button className={styles.button} onClick={() => void submitAll()}>Retry remaining answers</button></p>}</div>;
  }
  if (screen === "results") return <Results questions={questions} answers={answers} results={runtime.current.results} />;

  const question = questions[index];
	const option = answers[question.id];
  const answered = Object.keys(answers).filter((key) => answers[key] !== undefined).length;
  const options = question.options ?? [];
  const activeId = option ?? options[0]?.id;
  function choose(optionId: string) {
    setAnswers((old) => {
		const next = { ...old, [question.id]: optionId };
      answersRef.current = next;
      return next;
    });
  }
  function keydown(event: KeyboardEvent<HTMLButtonElement>, optionIndex: number) {
    const nextKeys = new Set(["ArrowDown", "ArrowRight"]);
    const previousKeys = new Set(["ArrowUp", "ArrowLeft"]);
    let target: number | undefined;
    if (nextKeys.has(event.key)) target = (optionIndex + 1) % options.length;
    else if (previousKeys.has(event.key)) target = (optionIndex - 1 + options.length) % options.length;
    else if (event.key === "Home") target = 0;
    else if (event.key === "End") target = options.length - 1;
    else if (event.key === " " || event.key === "Spacebar" || event.key === "Enter") {
      event.preventDefault();
      choose(options[optionIndex].id);
      return;
    }
    if (target !== undefined) {
      event.preventDefault();
      choose(options[target].id);
      optionRefs.current.get(options[target].id)?.focus();
    }
  }

  return <div className={styles.page}><header className={styles.header}><h1>Exam Mode</h1><p><strong aria-live="polite">Local timer: {clock(remaining)}</strong> · {answered}/{questions.length} answered · {Object.values(flags).filter(Boolean).length} flagged</p></header><section className={styles.card} aria-labelledby="exam-question"><p>Question {index + 1} of {questions.length}</p><p id="exam-question" className={styles.prompt}>{question.prompt}</p><div className={styles.options} role="radiogroup" aria-labelledby="exam-question">{options.map((item, optionIndex) => <button key={item.id} ref={(node) => { if (node) optionRefs.current.set(item.id, node); else optionRefs.current.delete(item.id); }} type="button" role="radio" aria-checked={option === item.id} tabIndex={item.id === activeId ? 0 : -1} className={styles.optionButton} data-option-state={option === item.id ? "selected" : "default"} onClick={() => choose(item.id)} onKeyDown={(event) => keydown(event, optionIndex)}><span className={styles.optionKey} aria-hidden="true">{item.label.charAt(0)}</span><span>{item.label}</span></button>)}</div><button type="button" className={styles.buttonSecondary} aria-pressed={Boolean(flags[question.id])} onClick={() => setFlags((old) => ({ ...old, [question.id]: !old[question.id] }))}>{flags[question.id] ? "Flagged" : "Flag for review"}</button></section><nav aria-label="Exam navigation" className={styles.form}><button type="button" className={styles.buttonSecondary} disabled={index === 0} onClick={() => setIndex(index - 1)}>Back</button><button type="button" className={styles.buttonSecondary} disabled={index === questions.length - 1} onClick={() => setIndex(index + 1)}>Next</button><button ref={submitTrigger} type="button" className={styles.button} onClick={() => setScreen("confirm")}>Submit all</button></nav>{screen === "confirm" && <section ref={dialog} className={styles.feedback} role="dialog" aria-modal="true" aria-labelledby="exam-confirm-heading"><h2 id="exam-confirm-heading">Submit all answers?</h2><p>{questions.length - answered} unanswered question{questions.length - answered === 1 ? "" : "s"} will be recorded locally as unanswered.</p><button ref={confirmButton} type="button" className={styles.button} onClick={() => void submitAll()}>Confirm submit</button><button type="button" className={styles.buttonSecondary} onClick={() => { setScreen("taking"); submitTrigger.current?.focus(); }}>Continue exam</button></section>}</div>;
}

function Results({ questions, answers, results }: { questions: LearnerQuestion[]; answers: Record<string, string | undefined>; results: ExamRuntime["results"] }) {
  const completed = Object.values(results);
  const awarded = completed.reduce((sum, result) => sum + result.awardedMarks, 0);
	const max = completed.reduce((sum, result) => sum + result.maxMarks, 0);
	// @ts-expect-error legacy MCQ-only result renderer narrows current string answers.
	return <div className={styles.page}><header className={styles.header}><h1>Exam results</h1><p><strong>Answered score: {awarded} / {max}</strong> · {completed.filter((result) => result.isCorrect).length} correct · {questions.length - completed.length} unanswered</p><p>Unanswered questions have no submitted attempt and are excluded from answered score.</p></header><ol className={styles.list}>{questions.map((question, index) => { const result = results[question.id]; const options = new Map(question.options?.map((item) => [item.id, item.label])); const answer = answers[question.id]; const selected = typeof answer === "string" ? answer : answer && "selectedOptionId" in answer ? answer.selectedOptionId : ""; return <li key={question.id} className={styles.card}><h2>Question {index + 1}</h2>{!result ? <p>Unanswered</p> : <section className={styles.feedback} aria-label={`Feedback for question ${index + 1}`}><p>{result.resultStatus === "pending_review" ? "Submitted for review" : result.isCorrect ? "Correct" : "Incorrect"} · {result.awardedMarks} / {result.maxMarks}</p><p><strong>Your answer:</strong> {options.get(selected) ?? "Submitted"}</p><p><strong>Correct answer:</strong> {options.get(result.correctOptionId) ?? result.correctOptionId}</p><h3>Explanation</h3><p>{result.feedback.correctExplanation}</p><ul>{result.feedback.incorrectExplanations.map((item) => <li key={item.optionId}>{options.get(item.optionId) ?? item.optionId}: {item.explanation}</li>)}</ul></section>}</li>; })}</ol></div>;
}
