import type { ReactNode } from "react";
import { AlertGlyph, InfoGlyph } from "./icons";
import styles from "./Message.module.css";

export type MessageTone = "info" | "success" | "warning" | "error" | "neutral";

interface MessageProps {
  tone: MessageTone;
  title: ReactNode;
  children?: ReactNode;
  /** Defaults to "alert" for error, "status" for the rest — pass to override. */
  role?: "alert" | "status";
}

function SuccessGlyph({ className }: { className?: string }) {
  return (
    <svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M4.8 8.2 7 10.4l4.2-4.6"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ErrorGlyph({ className }: { className?: string }) {
  return (
    <svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M5.6 5.6l4.8 4.8M10.4 5.6l-4.8 4.8" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  );
}

const ICONS: Record<MessageTone, (props: { className?: string }) => ReactNode> = {
  info: InfoGlyph,
  success: SuccessGlyph,
  warning: AlertGlyph,
  error: ErrorGlyph,
  neutral: InfoGlyph,
};

export function Message({ tone, title, children, role }: MessageProps) {
  const Icon = ICONS[tone];
  return (
    <div className={styles.msg} data-tone={tone} role={role ?? (tone === "error" ? "alert" : "status")}>
      <Icon className={styles.msg__icon} />
      <div>
        <div className={styles.msg__title}>{title}</div>
        {children && <div className={styles.msg__body}>{children}</div>}
      </div>
    </div>
  );
}
