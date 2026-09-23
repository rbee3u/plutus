import type { ButtonHTMLAttributes } from "react";
import { forwardRef } from "react";
import styles from "@/shared/ui/ActionButton.module.css";

/**
 * primary: the one main action of a screen. secondary: a bordered action.
 * ghost: a quiet action in a row or toolbar. danger: a destructive confirm.
 * dangerQuiet: a quiet action that turns red when hovered.
 */
export type ActionButtonTone = "primary" | "secondary" | "ghost" | "danger" | "dangerQuiet";

export interface ActionButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  tone?: ActionButtonTone;
}

export const ActionButton = forwardRef<HTMLButtonElement, ActionButtonProps>(function ActionButton(
  { type = "button", tone = "secondary", className, children, ...props },
  ref,
) {
  return (
    <button
      {...props}
      ref={ref}
      className={`${styles.button} ${styles[tone]} ${className ?? ""}`}
      type={type}
    >
      {children}
    </button>
  );
});
