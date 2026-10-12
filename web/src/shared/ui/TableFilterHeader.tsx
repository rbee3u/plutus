import { ChevronDown, Search, X } from "lucide-react";
import { useEffect, useId, useLayoutEffect, useRef, useState, type MouseEvent } from "react";
import { createPortal } from "react-dom";
import { iconSize } from "@/shared/icons/iconSizes";
import { TextInput } from "@/shared/ui/FormControls";
import { MenuItem } from "@/shared/ui/Menu";
import styles from "./TableFilterHeader.module.css";

export interface FilterOption {
  value: string;
  label: string;
}

export interface TableFilterHeaderProps {
  label: string;
  display: string;
  active: boolean;
  open: boolean;
  disabled?: boolean;
  type?: "select" | "search";
  onToggle: () => void;
  onClose: () => void;
  onClear: () => void;

  // Options for select type
  options?: FilterOption[];
  selectedValue?: string;
  onSelect?: (value: string) => void;

  // Options for search type
  searchDraft?: string;
  searchPlaceholder?: string;
  searchHelp?: string;
  onSearchChange?: (value: string) => void;
  onCompositionStart?: () => void;
  onCompositionEnd?: (value: string) => void;
}

/** A column label, current condition, and accessible filter outside table overflow. */
export function TableFilterHeader({
  label,
  display,
  active,
  open,
  disabled = false,
  type = "select",
  onToggle,
  onClose,
  onClear,
  options = [],
  selectedValue = "",
  onSelect,
  searchDraft = "",
  searchPlaceholder = "Search",
  searchHelp,
  onSearchChange,
  onCompositionStart,
  onCompositionEnd,
}: TableFilterHeaderProps) {
  const id = useId();
  const anchor = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const anchorPosition = useRef({ left: 0, bottom: 0 });
  const [position, setPosition] = useState({ left: 0, top: 0, maxHeight: 320 });
  const isSearch = type === "search";

  function handleSelect(event: MouseEvent<HTMLButtonElement>) {
    onSelect?.(event.currentTarget.value);
    anchor.current?.focus({ preventScroll: true });
  }

  useLayoutEffect(() => {
    if (!open || !anchor.current || !panel.current) return;
    const rect = anchor.current.getBoundingClientRect();
    anchorPosition.current = { left: rect.left, bottom: rect.bottom };
    const bounds = panel.current.getBoundingClientRect();
    const below = window.innerHeight - rect.bottom - 16;
    const above = rect.top - 16;
    const placeBelow = below >= bounds.height || below >= above;
    const maxHeight = Math.max(32, placeBelow ? below : above);
    setPosition({
      left: Math.max(8, Math.min(rect.left, window.innerWidth - bounds.width - 8)),
      top: placeBelow
        ? rect.bottom + 8
        : Math.max(8, rect.top - Math.min(bounds.height, maxHeight) - 8),
      maxHeight,
    });
    const target = panel.current.querySelector<HTMLElement>(
      isSearch ? "input" : '[aria-selected="true"]',
    );
    target?.focus({ preventScroll: true });
  }, [open, isSearch]);

  useEffect(() => {
    if (!open) return;
    function dismiss(event: Event) {
      const target = event.target as Node;
      if (event.type === "keydown") {
        if ((event as KeyboardEvent).key !== "Escape") return;
        event.preventDefault();
        onClose();
        anchor.current?.focus({ preventScroll: true });
        return;
      }
      if (panel.current?.contains(target)) return;
      if (event.type !== "scroll" && anchor.current?.contains(target)) return;
      if (event.type === "scroll" && anchor.current) {
        const rect = anchor.current.getBoundingClientRect();
        if (
          rect.left === anchorPosition.current.left &&
          rect.bottom === anchorPosition.current.bottom
        )
          return;
      }
      onClose();
      if (event.type === "scroll" || event.type === "resize")
        anchor.current?.focus({ preventScroll: true });
    }
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("focusin", dismiss);
    document.addEventListener("keydown", dismiss);
    document.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", dismiss);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("focusin", dismiss);
      document.removeEventListener("keydown", dismiss);
      document.removeEventListener("scroll", dismiss, true);
      window.removeEventListener("resize", dismiss);
    };
  }, [open, onClose]);

  return (
    <div className={styles.header}>
      <button
        ref={anchor}
        type="button"
        className={`${styles.trigger} ${active ? styles.active : ""}`}
        aria-label={
          isSearch
            ? active && display
              ? `Filter ${label}: ${display}`
              : `Search ${label}`
            : `Filter ${label}: ${display}`
        }
        aria-expanded={open}
        aria-haspopup={isSearch ? "dialog" : "listbox"}
        aria-controls={open ? id : undefined}
        disabled={disabled}
        onClick={onToggle}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown" && !open) {
            event.preventDefault();
            onToggle();
          }
        }}
      >
        <span className={styles.label}>{label}</span>
        {(!isSearch || active) && display ? <span className={styles.value}>{display}</span> : null}
        {isSearch ? (
          <Search size={iconSize.xs} className={styles.icon} aria-hidden="true" />
        ) : (
          <ChevronDown size={iconSize.xs} className={styles.icon} aria-hidden="true" />
        )}
      </button>
      {active ? (
        <button
          type="button"
          className={styles.clear}
          aria-label={`Clear ${label} filter`}
          onClick={() => {
            onClear();
            anchor.current?.focus({ preventScroll: true });
          }}
        >
          <X size={iconSize.xs} aria-hidden="true" />
        </button>
      ) : null}
      {open
        ? createPortal(
            <div
              ref={panel}
              id={id}
              role={isSearch ? "dialog" : "listbox"}
              aria-label={isSearch ? `Search ${label}` : `Choose ${label}`}
              className={`${styles.panel} ${isSearch ? styles.panelSearch : styles.panelSelect}`}
              style={position}
              onKeyDown={(event) => {
                if (isSearch || !["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key))
                  return;
                const items = [
                  ...event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="option"]'),
                ];
                const index = items.findIndex((item) => item === document.activeElement);
                const last = items.length - 1;
                const next =
                  event.key === "Home"
                    ? 0
                    : event.key === "End"
                      ? last
                      : event.key === "ArrowDown"
                        ? (index + 1) % items.length
                        : index <= 0
                          ? last
                          : index - 1;
                event.preventDefault();
                items[next]?.focus();
              }}
            >
              {isSearch ? (
                <form
                  className={styles.search}
                  onSubmit={(event) => {
                    event.preventDefault();
                    onClose();
                  }}
                >
                  <label htmlFor={`${id}-input`}>Search {label}</label>
                  <TextInput
                    id={`${id}-input`}
                    value={searchDraft}
                    placeholder={searchPlaceholder}
                    autoComplete="off"
                    onChange={(event) => onSearchChange?.(event.target.value)}
                    onCompositionStart={onCompositionStart}
                    onCompositionEnd={(event) => onCompositionEnd?.(event.currentTarget.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" && event.nativeEvent.isComposing)
                        event.preventDefault();
                    }}
                  />
                  {searchHelp ? <p>{searchHelp}</p> : null}
                </form>
              ) : (
                options.map((option) => (
                  <MenuItem
                    key={option.value}
                    role="option"
                    tabIndex={selectedValue === option.value ? 0 : -1}
                    checked={selectedValue === option.value}
                    title={option.label}
                    value={option.value}
                    onClick={handleSelect}
                  >
                    {option.label}
                  </MenuItem>
                ))
              )}
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
