"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { ApiError, createExamAttempt, listExamQuestions, listExamSyllabuses, submitExamAttempt } from "./api-client";
import { finalizeExam, type ExamRuntime } from "./finalization";
import styles from "../practice/styles.module.css";
import type { LearnerQuestion, LearnerSyllabus } from "./types";

type Screen = "setup" | "taking" | "confirm" | "submitting" | "retry" | "results";
const DURATION_SECONDS = 30 * 60;

function errorMessage(error: unknown) {
  return error instanceof ApiError ? error.message : "Exam could not be completed. Retry remaining answers.";
}
function label(s: LearnerSyllabus) { return s.track ? `${s.displayName} (${s.track})` : s.displayName; }
function clock(seconds: number) { return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`; }

export function ExamWorkspace() {
  const [syllabuses, setSyllabuses] = useState<LearnerSyllabus[] | null>(null);
  const [loadError, setLoadError] = useState<string>();
  const [syllabusId, setSyllabusId] = useState("");
  const [nodeId, setNodeId] = useState("");
  const [count, setCount] = useState("2");
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
  const optionRefs = useRef(new Map<string, HTMLButtonElement>());

  async function loadSyllabuses() {
    setLoadError(undefined);
    try { setSyllabuses((await listExamSyllabuses()).items); }
    catch (error) { setLoadError(errorMessage(error)); }
  }
  useEffect(() => { void loadSyllabuses(); }, []);

  useEffect(() => {
    if (screen !== "taking" || !deadline.current) return;
    const timer = window.setInterval(() => {
      const next = Math.max(0, Math.ceil((deadline.current! - Date.now()) / 1000));
      setRemaining(next);
      if (next === 0) { window.clearInterval(timer); void submitAll(true); }
    }, 1000);
    return () => window.clearInterval(timer);
  // submitAll is intentionally invoked only when timer reaches zero; client-local timer is disclosed.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen]);

  async function start(event: React.FormEvent) {
    event.preventDefault();
    setSetupMessage(undefined);
    try {
      const available = (await listExamQuestions(syllabusId, nodeId.trim() || undefined)).items
        .filter((q) => q.responseType === "multiple_choice" && Boolean(q.options?.length));
      const requested = Number(count);
      if (available.length < requested) {
        setSetupMessage(`Need ${requested} eligible multiple-choice questions. Only ${available.length} available.`);
        return;
      }
      setQuestions(available.slice(0, requested));
      setAnswers({}); setFlags({}); setIndex(0); setRemaining(DURATION_SECONDS); runtime.current = { attempts: {}, results: {} };
      deadline.current = Date.now() + DURATION_SECONDS * 1000;
      setScreen("taking");
    } catch (error) { setSetupMessage(errorMessage(error)); }
  }

  async function submitAll(expired = false) {
    setScreen("submitting"); setFinalError(undefined);
    try {
      await finalizeExam(questions, answers, runtime.current, {
        createAttempt: createExamAttempt,
        submitAttempt: submitExamAttempt,
      }, (done, total) => setProgress(`Marking ${done} of ${total} answered question${total === 1 ? "" : "s"}…`));
      setScreen("results");
    } catch (error) {
      setFinalError(`${expired ? "Time ended. " : ""}${errorMessage(error)}`);
      setScreen("retry");
    }
  }

  if (syllabuses === null) return <div className={styles.page}><h1>Exam Mode</h1>{loadError ? <p role="alert" className={styles.bannerError}>{loadError} <button className={styles.buttonSecondary} onClick={() => void loadSyllabuses()}>Retry</button></p> : <p role="status">Loading syllabuses…</p>}</div>;
  if (screen === "setup") return <div className={styles.page}><header className={styles.header}><h1>Exam Mode</h1><p>Local MVP. 30-minute countdown runs in this browser only; refreshing loses exam progress.</p></header><form className={styles.form} onSubmit={start}><div className={styles.field}><label htmlFor="exam-syllabus">Syllabus</label><select id="exam-syllabus" value={syllabusId} onChange={(e) => setSyllabusId(e.target.value)} required><option value="" disabled>Select a syllabus</option>{syllabuses.map((s) => <option key={s.id} value={s.id}>{label(s)}</option>)}</select></div><div className={styles.field}><label htmlFor="exam-node">Curriculum node ID (optional)</label><input id="exam-node" value={nodeId} onChange={(e) => setNodeId(e.target.value)} /></div><div className={styles.field}><label htmlFor="exam-count">Questions</label><select id="exam-count" value={count} onChange={(e) => setCount(e.target.value)}>{[2,3,4,5,6,7,8,9,10].map((n) => <option key={n}>{n}</option>)}</select></div><button className={styles.button} disabled={!syllabusId}>Start exam</button></form>{setupMessage && <p role="alert" className={styles.bannerError}>{setupMessage}</p>}</div>;
  if (screen === "submitting" || screen === "retry") return <div className={styles.page}><h1>Finalising exam</h1><p role="status">{progress || "Preparing answers…"}</p>{screen === "retry" && <p role="alert" className={styles.bannerError}>{finalError} <button className={styles.button} onClick={() => void submitAll()}>Retry remaining answers</button></p>}</div>;
  if (screen === "results") return <Results questions={questions} answers={answers} results={runtime.current.results} />;

  const question = questions[index]; const option = answers[question.id]; const answered = Object.keys(answers).filter((key) => answers[key]).length;
  const options = question.options ?? [];
  const activeId = option ?? options[0]?.id;
  function choose(optionId: string) { setAnswers((old) => ({ ...old, [question.id]: optionId })); }
  function keydown(event: KeyboardEvent<HTMLButtonElement>, optionIndex: number) {
    const nextKeys = new Set(["ArrowDown", "ArrowRight"]); const previousKeys = new Set(["ArrowUp", "ArrowLeft"]);
    let target: number | undefined;
    if (nextKeys.has(event.key)) target = (optionIndex + 1) % options.length;
    else if (previousKeys.has(event.key)) target = (optionIndex - 1 + options.length) % options.length;
    else if (event.key === "Home") target = 0;
    else if (event.key === "End") target = options.length - 1;
    else if (event.key === " " || event.key === "Spacebar" || event.key === "Enter") { event.preventDefault(); choose(options[optionIndex].id); return; }
    if (target !== undefined) { event.preventDefault(); choose(options[target].id); optionRefs.current.get(options[target].id)?.focus(); }
  }
  return <div className={styles.page}><header className={styles.header}><h1>Exam Mode</h1><p><strong aria-live="polite">Local timer: {clock(remaining)}</strong> · {answered}/{questions.length} answered · {Object.values(flags).filter(Boolean).length} flagged</p></header><section className={styles.card} aria-labelledby="exam-question"><p>Question {index + 1} of {questions.length}</p><p id="exam-question" className={styles.prompt}>{question.prompt}</p><div className={styles.options} role="radiogroup" aria-labelledby="exam-question">{options.map((item, optionIndex) => <button key={item.id} ref={(node) => { if (node) optionRefs.current.set(item.id, node); else optionRefs.current.delete(item.id); }} type="button" role="radio" aria-checked={option === item.id} tabIndex={item.id === activeId ? 0 : -1} className={styles.optionButton} data-option-state={option === item.id ? "selected" : "default"} onClick={() => choose(item.id)} onKeyDown={(event) => keydown(event, optionIndex)}><span className={styles.optionKey} aria-hidden="true">{item.label.charAt(0)}</span><span>{item.label}</span></button>)}</div><button type="button" className={styles.buttonSecondary} aria-pressed={Boolean(flags[question.id])} onClick={() => setFlags((old) => ({ ...old, [question.id]: !old[question.id] }))}>{flags[question.id] ? "Flagged" : "Flag for review"}</button></section><nav aria-label="Exam navigation" className={styles.form}><button className={styles.buttonSecondary} disabled={index === 0} onClick={() => setIndex(index - 1)}>Back</button><button className={styles.buttonSecondary} disabled={index === questions.length - 1} onClick={() => setIndex(index + 1)}>Next</button><button className={styles.button} onClick={() => setScreen("confirm")}>Submit all</button></nav>{screen === "confirm" && <section className={styles.feedback} role="dialog" aria-modal="true" aria-label="Submit exam confirmation"><h2>Submit all answers?</h2><p>{questions.length - answered} unanswered question{questions.length - answered === 1 ? "" : "s"} will be recorded locally as unanswered.</p><button className={styles.button} onClick={() => void submitAll()}>Confirm submit</button><button className={styles.buttonSecondary} onClick={() => setScreen("taking")}>Continue exam</button></section>}</div>;
}

function Results({ questions, answers, results }: { questions: LearnerQuestion[]; answers: Record<string, string | undefined>; results: ExamRuntime["results"] }) {
  const completed = Object.values(results); const awarded = completed.reduce((sum, result) => sum + result.awardedMarks, 0); const max = completed.reduce((sum, result) => sum + result.maxMarks, 0);
  return <div className={styles.page}><header className={styles.header}><h1>Exam results</h1><p><strong>Score: {awarded} / {max}</strong> · {completed.filter((r) => r.isCorrect).length} correct · {questions.length - completed.length} unanswered</p></header><ol className={styles.list}>{questions.map((question, index) => { const result = results[question.id]; const options = new Map(question.options?.map((x) => [x.id, x.label])); return <li key={question.id} className={styles.card}><h2>Question {index + 1}</h2>{!result ? <p>Unanswered</p> : <section className={styles.feedback} aria-label={`Feedback for question ${index + 1}`}><p>{result.isCorrect ? "Correct" : "Incorrect"} · {result.awardedMarks} / {result.maxMarks}</p><p><strong>Your answer:</strong> {options.get(answers[question.id] ?? "") ?? "Unknown"}</p><p><strong>Correct answer:</strong> {options.get(result.correctOptionId) ?? result.correctOptionId}</p><h3>Explanation</h3><p>{result.feedback.correctExplanation}</p><ul>{result.feedback.incorrectExplanations.map((item) => <li key={item.optionId}>{options.get(item.optionId) ?? item.optionId}: {item.explanation}</li>)}</ul></section>}</li>; })}</ol></div>;
}
