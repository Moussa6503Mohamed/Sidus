import type { CSSProperties } from "react";
import styles from "./Skeleton.module.css";

interface SkeletonProps {
  width?: string;
  height?: string;
  style?: CSSProperties;
}

/** Decorative shimmer only — the loading state's accessible name comes from a sibling
 * `role="status"` text node, so screen readers announce once, not per skeleton frame. Stops
 * animating under `prefers-reduced-motion: reduce` (global rule in tokens.css). */
export function Skeleton({ width, height, style }: SkeletonProps) {
  return <div className={styles.skel} style={{ width, height, ...style }} aria-hidden="true" />;
}
