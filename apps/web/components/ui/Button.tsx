import type { ButtonHTMLAttributes } from "react";
import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "default" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

/** One primary action per view is a product decision made by callers, not enforced here.
 * Disabled buttons should keep a label that states the unmet precondition rather than a bare
 * greyed-out verb (caller's responsibility — this component only supplies the visual treatment). */
export function Button({ variant = "secondary", size = "default", className, type = "button", ...rest }: ButtonProps) {
  const classes = [styles.btn, styles[variant], size !== "default" ? styles[size] : "", className ?? ""]
    .filter(Boolean)
    .join(" ");
  return <button type={type} className={classes} {...rest} />;
}
