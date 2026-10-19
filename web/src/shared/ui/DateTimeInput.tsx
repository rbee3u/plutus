import type { ChangeEvent, InputHTMLAttributes } from "react";
import { forwardRef, useRef } from "react";
import { Calendar } from "lucide-react";
import { iconSize } from "@/shared/icons/iconSizes";
import { toDateTimeDisplay, toDateTimeIso } from "@/shared/lib/datetime";
import formStyles from "@/shared/ui/FormControls.module.css";
import styles from "@/shared/ui/DateTimeInput.module.css";

export interface DateTimeInputProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "type" | "value" | "onChange"
> {
  value: string;
  onChange: (value: string) => void;
  pickerAriaLabel?: string;
}

export const DateTimeInput = forwardRef<HTMLInputElement, DateTimeInputProps>(
  function DateTimeInput(
    {
      className,
      style,
      disabled,
      placeholder = "YYYY-MM-DD HH:mm",
      value,
      onChange,
      pickerAriaLabel = "Open calendar picker",
      ...props
    },
    ref,
  ) {
    const pickerRef = useRef<HTMLInputElement>(null);

    const displayValue = toDateTimeDisplay(value);
    const pickerValue = toDateTimeIso(value);

    function handleTextChange(event: ChangeEvent<HTMLInputElement>) {
      onChange(toDateTimeDisplay(event.target.value));
    }

    function handlePickerChange(event: ChangeEvent<HTMLInputElement>) {
      const next = event.target.value;
      if (!next) return;
      onChange(toDateTimeDisplay(next));
    }

    function openPicker() {
      if (disabled) return;
      try {
        if (pickerRef.current && "showPicker" in pickerRef.current) {
          pickerRef.current.showPicker();
          return;
        }
      } catch {
        // Fall back to focus if showPicker is unsupported or restricted.
      }
      pickerRef.current?.focus();
    }

    return (
      <div className={`${styles.container} ${className ?? ""}`} style={style}>
        <input
          {...props}
          ref={ref}
          type="text"
          disabled={disabled}
          placeholder={placeholder}
          value={displayValue}
          onChange={handleTextChange}
          className={`${formStyles.control} ${styles.input}`}
        />
        <button
          type="button"
          disabled={disabled}
          className={styles.pickerButton}
          onClick={openPicker}
          aria-label={pickerAriaLabel}
          title={pickerAriaLabel}
        >
          <Calendar size={iconSize.sm} />
        </button>
        <input
          ref={pickerRef}
          type="datetime-local"
          disabled={disabled}
          tabIndex={-1}
          aria-hidden="true"
          className={styles.hiddenPicker}
          value={pickerValue}
          onChange={handlePickerChange}
        />
      </div>
    );
  },
);
