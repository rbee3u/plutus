import { ChevronDown, Search, X } from "lucide-react";
import { useEffect, useId, useLayoutEffect, useRef, useState, type MouseEvent } from "react";
import { createPortal } from "react-dom";
import type { HoldingsFilterKey, HoldingsFilters } from "./useHoldingsFilters";
import { iconSize } from "@/shared/icons/iconSizes";
import { TextInput } from "@/shared/ui/FormControls";
import { MenuItem } from "@/shared/ui/Menu";
import styles from "./HoldingsFilterHeader.module.css";

/** A column label, current condition, and accessible filter outside the table clip. */
export function HoldingsFilterHeader({
  label,
  filterKey,
  filters,
}: {
  label: string;
  filterKey: HoldingsFilterKey;
  filters: HoldingsFilters;
}) {
  const id = useId();
  const anchor = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const anchorPosition = useRef({ left: 0, bottom: 0 });
  const [position, setPosition] = useState({ left: 0, top: 0, maxHeight: 320 });
  const open = filters.open === filterKey;
  const search = filterKey === "symbol";
  const value = search ? filters.symbolDraft : filters.values[filterKey];
  const display = !value
    ? "All"
    : search
      ? value
      : filterKey === "platform"
        ? filters.platformName
        : filters.identityName;
  const options = [
    { value: "", label: "All" },
    ...(filterKey === "platform" ? filters.platformOptions : filters.identityOptions),
  ];
  const close = filters.close;
  function select(event: MouseEvent<HTMLButtonElement>) {
    filters.change({ [filterKey]: event.currentTarget.value });
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
      search ? "input" : '[aria-selected="true"]',
    );
    target?.focus({ preventScroll: true });
  }, [open, search]);

  useEffect(() => {
    if (!open) return;
    function dismiss(event: Event) {
      const target = event.target as Node;
      if (event.type === "keydown") {
        if ((event as KeyboardEvent).key !== "Escape") return;
        event.preventDefault();
        close();
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
      close();
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
  }, [open, close]);

  return (
    <div className={styles.header}>
      <button
        ref={anchor}
        type="button"
        className={`${styles.trigger} ${value ? styles.active : ""}`}
        aria-label={`Filter ${label}: ${display}`}
        aria-expanded={open}
        aria-haspopup={search ? "dialog" : "listbox"}
        aria-controls={open ? id : undefined}
        disabled={!search && !filters.candidatesReady}
        onClick={() => filters.toggle(filterKey)}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown" && !open) {
            event.preventDefault();
            filters.toggle(filterKey);
          }
        }}
      >
        <span className={styles.label}>
          {label}
          {search ? (
            <Search size={iconSize.sm} aria-hidden="true" />
          ) : (
            <ChevronDown size={iconSize.sm} aria-hidden="true" />
          )}
        </span>
        <span className={styles.value}>{display}</span>
      </button>
      {value ? (
        <button
          type="button"
          className={styles.clear}
          aria-label={`Clear ${label} filter`}
          onClick={() => {
            filters.change({ [filterKey]: "" });
            anchor.current?.focus({ preventScroll: true });
          }}
        >
          <X size={iconSize.sm} aria-hidden="true" />
        </button>
      ) : null}
      {open
        ? createPortal(
            <div
              ref={panel}
              id={id}
              role={search ? "dialog" : "listbox"}
              aria-label={search ? `Search ${label}` : `Choose ${label}`}
              className={styles.panel}
              style={position}
              onKeyDown={(event) => {
                if (search || !["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
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
              {search ? (
                <form
                  className={styles.search}
                  onSubmit={(event) => {
                    event.preventDefault();
                    close();
                  }}
                >
                  <label htmlFor={`${id}-input`}>Search {label}</label>
                  <TextInput
                    id={`${id}-input`}
                    value={filters.symbolDraft}
                    placeholder="Contains, e.g. BTC"
                    autoComplete="off"
                    onChange={(event) => filters.editSymbol(event.target.value)}
                    onCompositionStart={filters.beginComposition}
                    onCompositionEnd={(event) => filters.endComposition(event.currentTarget.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" && event.nativeEvent.isComposing)
                        event.preventDefault();
                    }}
                  />
                  <p>Matches part of a symbol, ignoring case.</p>
                </form>
              ) : (
                options.map((option) => (
                  <MenuItem
                    key={option.value}
                    role="option"
                    tabIndex={value === option.value ? 0 : -1}
                    checked={value === option.value}
                    title={option.label}
                    value={option.value}
                    onClick={select}
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
