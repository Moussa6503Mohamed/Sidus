"use client";

import { useEffect, useState } from "react";
import styles from "./ThemeToggle.module.css";

export type ThemePreference = "system" | "light" | "dark";

const STORAGE_KEY = "sidus-theme";
const ORDER: ThemePreference[] = ["system", "light", "dark"];
const LABELS: Record<ThemePreference, string> = { system: "System", light: "Light", dark: "Dark" };

function applyTheme(preference: ThemePreference): void {
  const root = document.documentElement;
  if (preference === "system") {
    root.removeAttribute("data-theme");
  } else {
    root.setAttribute("data-theme", preference);
  }
}

function readStoredPreference(): ThemePreference {
  const stored = window.localStorage.getItem(STORAGE_KEY);
  return stored === "light" || stored === "dark" ? stored : "system";
}

/** Cycles system -> light -> dark -> system, persisting the explicit choice (or clearing it for
 * "system") to localStorage. Renders "System" on the server and on first client render — the
 * real stored value is only known after mount — so hydration never mismatches. */
export function ThemeToggle() {
  const [preference, setPreference] = useState<ThemePreference>("system");
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setPreference(readStoredPreference());
    setMounted(true);
  }, []);

  function cycle() {
    const next = ORDER[(ORDER.indexOf(preference) + 1) % ORDER.length];
    setPreference(next);
    applyTheme(next);
    if (next === "system") {
      window.localStorage.removeItem(STORAGE_KEY);
    } else {
      window.localStorage.setItem(STORAGE_KEY, next);
    }
  }

  const shown = mounted ? preference : "system";

  return (
    <button
      type="button"
      className={styles.toggle}
      onClick={cycle}
      aria-label={`Theme: ${LABELS[shown]}. Activate to change theme.`}
    >
      <ThemeIcon preference={shown} />
      <span className={styles.toggle__label}>{LABELS[shown]}</span>
    </button>
  );
}

function ThemeIcon({ preference }: { preference: ThemePreference }) {
  if (preference === "light") {
    return (
      <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
        <circle cx="8" cy="8" r="3.4" fill="none" stroke="currentColor" strokeWidth="1.5" />
        <g stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          <path d="M8 1.4v1.6M8 13v1.6M1.4 8h1.6M13 8h1.6" />
          <path d="M3.4 3.4l1.2 1.2M11.4 11.4l1.2 1.2M3.4 12.6l1.2-1.2M11.4 4.6l1.2-1.2" />
        </g>
      </svg>
    );
  }
  if (preference === "dark") {
    return (
      <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
        <path
          d="M13.6 9.9A5.6 5.6 0 1 1 6.1 2.4a4.6 4.6 0 0 0 7.5 7.5z"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
      </svg>
    );
  }
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
      <rect x="1.4" y="3" width="13.2" height="8.4" rx="1.2" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M5.4 14h5.2M8 11.4V14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
