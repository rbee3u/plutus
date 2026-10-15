import type { InputHTMLAttributes } from "react";
import { forwardRef, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { iconSize } from "@/shared/icons/iconSizes";
import formStyles from "@/shared/ui/FormControls.module.css";
import styles from "@/shared/ui/PasswordInput.module.css";

interface PasswordInputProps extends InputHTMLAttributes<HTMLInputElement> {
  fieldName?: string;
}

export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(
  function PasswordInput({ className, disabled, fieldName = "password", ...props }, ref) {
    const [visible, setVisible] = useState(false);
    const toggleLabel = `${visible ? "Hide" : "Show"} ${fieldName}`;

    return (
      <div className={styles.container}>
        <input
          {...props}
          ref={ref}
          type={visible ? "text" : "password"}
          disabled={disabled}
          className={`${formStyles.control} ${styles.input} ${className ?? ""}`}
        />
        <button
          type="button"
          disabled={disabled}
          className={styles.toggle}
          onClick={() => setVisible((prev) => !prev)}
          aria-label={toggleLabel}
          title={toggleLabel}
        >
          {visible ? <EyeOff size={iconSize.sm} /> : <Eye size={iconSize.sm} />}
        </button>
      </div>
    );
  },
);
