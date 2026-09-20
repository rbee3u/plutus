import type { ButtonHTMLAttributes, ReactNode, Ref } from "react";
import { forwardRef } from "react";
import { ActionButton, type ActionButtonTone } from "@/shared/ui/ActionButton";
import styles from "@/shared/ui/IconButton.module.css";

type IconButtonProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> & {
  label: string;
  children: ReactNode;
  tone?: Extract<ActionButtonTone, "ghost" | "dangerQuiet" | "danger">;
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { label, children, className, tone = "ghost", ...props },
  ref: Ref<HTMLButtonElement>,
) {
  return (
    <ActionButton
      {...props}
      ref={ref}
      className={`${styles.button} ${className ?? ""}`}
      tone={tone}
      type="button"
      aria-label={label}
      title={label}
    >
      {children}
    </ActionButton>
  );
});
