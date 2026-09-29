import type { KeyboardEvent } from "react";
import { useRef } from "react";
import styles from "@/shared/ui/SegmentedControl.module.css";

export interface SegmentedControlOption<T extends string = string> {
  value: T;
  label: string;
  disabled?: boolean;
}

export interface SegmentedControlProps<T extends string = string> {
  value: T;
  onChange: (value: T) => void;
  options: readonly SegmentedControlOption<T>[];
  label: string;
  className?: string;
  size?: "sm" | "md";
}

export function SegmentedControl<T extends string = string>({
  value,
  onChange,
  options,
  label,
  className,
  size = "md",
}: SegmentedControlProps<T>) {
  const buttonRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const enabledOptions = options.filter((opt) => !opt.disabled);

  function focusOption(targetValue: T) {
    const targetIndex = options.findIndex((opt) => opt.value === targetValue);
    if (targetIndex >= 0) {
      buttonRefs.current[targetIndex]?.focus();
    }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>, currentValue: T) {
    if (enabledOptions.length <= 1) return;

    const currentIndex = enabledOptions.findIndex((opt) => opt.value === currentValue);
    if (currentIndex === -1) return;

    let nextIndex = -1;

    switch (event.key) {
      case "ArrowRight":
      case "ArrowDown":
        nextIndex = (currentIndex + 1) % enabledOptions.length;
        break;
      case "ArrowLeft":
      case "ArrowUp":
        nextIndex = (currentIndex - 1 + enabledOptions.length) % enabledOptions.length;
        break;
      case "Home":
        nextIndex = 0;
        break;
      case "End":
        nextIndex = enabledOptions.length - 1;
        break;
      default:
        return;
    }

    event.preventDefault();
    const nextOption = enabledOptions[nextIndex];
    if (nextOption && nextOption.value !== value) {
      onChange(nextOption.value);
      focusOption(nextOption.value);
    }
  }

  return (
    <div
      role="tablist"
      aria-label={label}
      className={`${styles.track} ${size === "sm" ? styles.sm : ""} ${className ?? ""}`}
    >
      {options.map((option, index) => {
        const isSelected = option.value === value;
        return (
          <button
            key={option.value}
            ref={(el) => {
              buttonRefs.current[index] = el;
            }}
            type="button"
            role="tab"
            aria-selected={isSelected}
            tabIndex={isSelected ? 0 : -1}
            disabled={option.disabled}
            className={styles.segment}
            onClick={() => {
              if (!option.disabled && option.value !== value) {
                onChange(option.value);
              }
            }}
            onKeyDown={(e) => handleKeyDown(e, option.value)}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
