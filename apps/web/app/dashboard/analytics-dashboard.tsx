"use client";

import { useEffect, useState } from "react";
import type { LearnerAnalyticsSummary } from "@sidus/shared";
import styles from "./dashboard.module.css";

type State = { kind: "loading" } | { kind: "error" } | { kind: "ready"; data: LearnerAnalyticsSummary };

export function AnalyticsDashboard() {
  const [state, setState] = useState<State>({ kind: "loading" });
  const load = async () => {
    setState({ kind: "loading" });
    try {
      const response = await fetch("/api/learner/analytics", { cache: "no-store" });
      if (!response.ok) throw new Error("analytics unavailable");
      setState({ kind: "ready", data: await response.json() as LearnerAnalyticsSummary });
    } catch { setState({ kind: "error" }); }
  };
  useEffect(() => { void load(); }, []);
  if (state.kind === "loading") return <p aria-live="polite" className={styles.muted}>Loading your learning progress…</p>;
  if (state.kind === "error") return <section className={styles.notice} role="alert"><p>Progress is temporarily unavailable.</p><button onClick={() => void load()}>Try again</button></section>;
  const { data } = state;
  if (data.scoredItems === 0 && data.pendingMarking === 0 && data.withheldMarking === 0) return <section className={styles.notice}><h2>Your progress will appear here</h2><p>Complete a Practice or Exam question to begin building your personal record.</p></section>;
  return <section className={styles.analytics} aria-label="Learning progress">
    <div className={styles.metrics}><Metric label="Scored items" value={String(data.scoredItems)} /><Metric label="Marks" value={`${data.awardedMarks} / ${data.possibleMarks}`} /><Metric label="Awaiting marking" value={String(data.pendingMarking)} /><Metric label="Withheld" value={String(data.withheldMarking)} /></div>
    {data.pendingMarking > 0 ? <p className={styles.muted}>Some written responses are awaiting automated marking and are not included in your score.</p> : null}
    {data.withheldMarking > 0 ? <p className={styles.muted}>Some written responses could not be scored automatically and are shown separately.</p> : null}
    {data.modules.length > 0 ? <section><h2>Progress by module</h2><ul className={styles.list}>{data.modules.map((module) => <li key={module.moduleId}><strong>{module.moduleLabel}</strong><span>{module.awardedMarks} / {module.possibleMarks} marks · {module.scoredItems} scored</span></li>)}</ul></section> : null}
    {data.recentActivity.length > 0 ? <section><h2>Recent activity</h2><ul className={styles.list}>{data.recentActivity.map((event, index) => <li key={`${event.occurredAt}-${index}`}><strong>{event.moduleLabel}</strong><span>{event.eventType === "pending_marking" ? "Awaiting marking" : event.eventType === "automated_marking_withheld" ? "Marking withheld" : `${event.awardedMarks} / ${event.maxMarks} marks`}</span></li>)}</ul></section> : null}
  </section>;
}
function Metric({ label, value }: { label: string; value: string }) { return <div className={styles.metric}><span>{label}</span><strong>{value}</strong></div>; }
