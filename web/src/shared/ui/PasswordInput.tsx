import type { InputHTMLAttributes } from "react";
import { forwardRef, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { iconSize } from "@/shared/icons/iconSizes";
import formStyles from "@/shared/ui/FormControls.module.css";
import styles from "@/shared/ui/PasswordInput.module.css";

export const PasswordInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function PasswordInput({ className, disabled, ...props }, ref) {
    const [visible, setVisible] = useState(false);

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
          aria-label={visible ? "Hide password" : "Show password"}
          title={visible ? "Hide password" : "Show password"}
        >
          {visible ? <EyeOff size={iconSize.sm} /> : <Eye size={iconSize.sm} />}
        </button>
      </div>
    );
  },
);
