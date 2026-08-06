import styles from "./Logo.module.css";

export type LogoVariant = "lockup" | "mark" | "icon";
export type LogoSize = "sm" | "md" | "lg";

interface LogoProps {
  /** "lockup" = mark + wordmark, "mark" = A* glyph only, "icon" = delta-A only (favicon-scale). */
  variant?: LogoVariant;
  size?: LogoSize;
  /** Renders the mark in white for navy/brand-colored grounds. */
  inverse?: boolean;
  className?: string;
}

/**
 * Original geometric navy delta-A with a triangular counter, followed by a six-bar asterisk —
 * read together as "A*", the top Cambridge grade. Single-color `currentColor` SVG, no external
 * asset. See docs/sidus-observatory-design-system.md for clearspace/never-do rules.
 */
export function Logo({ variant = "lockup", size = "md", inverse = false, className }: LogoProps) {
  const rootClassName = [
    styles.logo,
    styles[`logo--${size}`],
    inverse ? styles["logo--inverse"] : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");

  if (variant === "icon") {
    return (
      <svg
        className={rootClassName}
        viewBox="0 0 26 32"
        role="img"
        aria-label="Sidus"
      >
        <path
          fill="currentColor"
          fillRule="evenodd"
          d="M13 2.2 L25.6 30 H19.4 L17.6 25.6 H8.4 L6.6 30 H0.4 Z M13 12.4 L10.1 20.2 H15.9 Z"
        />
      </svg>
    );
  }

  const mark = (
    <svg className={styles.logo__mark} viewBox="0 0 42 32" role="img" aria-label="Sidus">
      <path
        fill="currentColor"
        fillRule="evenodd"
        d="M13 2.2 L25.6 30 H19.4 L17.6 25.6 H8.4 L6.6 30 H0.4 Z M13 12.4 L10.1 20.2 H15.9 Z"
      />
      <g fill="currentColor">
        <rect x="32.2" y="1.4" width="2.6" height="11.6" rx="1.3" />
        <rect x="32.2" y="1.4" width="2.6" height="11.6" rx="1.3" transform="rotate(60 33.5 7.2)" />
        <rect x="32.2" y="1.4" width="2.6" height="11.6" rx="1.3" transform="rotate(120 33.5 7.2)" />
      </g>
    </svg>
  );

  if (variant === "mark") {
    return <span className={rootClassName}>{mark}</span>;
  }

  return (
    <span className={rootClassName}>
      {mark}
      <span className={styles.logo__word}>Sidus</span>
    </span>
  );
}
