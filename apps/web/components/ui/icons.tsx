import type { StatusIcon } from "@/lib/design/status";

interface IconProps {
  className?: string;
}

/** Lifecycle-status glyphs. Always `aria-hidden` — the accessible name is the status text label
 * that renders beside them, never the icon alone. */
export function StatusGlyph({ icon, className }: { icon: StatusIcon; className?: string }) {
  const common = { width: 13, height: 13, viewBox: "0 0 16 16", "aria-hidden": true as const, className };
  switch (icon) {
    case "pencil":
      return (
        <svg {...common}>
          <path
            d="M11.2 1.9 14.1 4.8 5.6 13.3H2.7v-2.9z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
          />
        </svg>
      );
    case "clock":
      return (
        <svg {...common}>
          <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.6" />
          <path d="M8 4.4V8l2.6 1.7" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      );
    case "check-circle":
      return (
        <svg {...common}>
          <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.6" />
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
    case "shield-check":
      return (
        <svg {...common}>
          <path
            d="M8 1.4 13.6 3.6v4.1c0 3.2-2.3 5.7-5.6 6.9-3.3-1.2-5.6-3.7-5.6-6.9V3.6z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinejoin="round"
          />
          <path
            d="M5.6 7.9 7.4 9.7l3.2-3.6"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      );
    case "cross-circle":
      return (
        <svg {...common}>
          <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.6" />
          <path d="M5.6 5.6l4.8 4.8M10.4 5.6l-4.8 4.8" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
        </svg>
      );
    case "archive":
      return (
        <svg {...common}>
          <rect x="2.2" y="4.4" width="11.6" height="9.2" rx="1.4" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M1.2 4.4h13.6M6.2 8.4h3.6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      );
    default:
      return null;
  }
}

export function CheckGlyph({ className }: IconProps) {
  return (
    <svg width="12" height="12" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <path
        d="M3.2 8.4 6.6 11.8 12.8 4.6"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.1"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function CrossGlyph({ className }: IconProps) {
  return (
    <svg width="12" height="12" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <path d="M4.2 4.2l7.6 7.6M11.8 4.2l-7.6 7.6" stroke="currentColor" strokeWidth="2.1" strokeLinecap="round" />
    </svg>
  );
}

export function TargetGlyph({ className }: IconProps) {
  return (
    <svg width="12" height="12" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.6" />
      <circle cx="8" cy="8" r="3" fill="currentColor" />
    </svg>
  );
}

export function AlertGlyph({ className }: IconProps) {
  return (
    <svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <path d="M8 1.8 15 14H1z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
      <path d="M8 6v3.4M8 11.4v.2" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  );
}

export function InfoGlyph({ className }: IconProps) {
  return (
    <svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true" className={className}>
      <circle cx="8" cy="8" r="6.4" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M8 7.2v4M8 4.6v.2" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  );
}
