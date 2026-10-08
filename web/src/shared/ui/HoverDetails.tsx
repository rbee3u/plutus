import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import styles from "./HoverDetails.module.css";

interface PositionState {
  left: number;
  top: number;
  placement: "below" | "above";
  arrowOffset: number;
}

/** Rich details available to pointer, keyboard, and touch users outside scroll clips. */
export function HoverDetails({
  label,
  children,
  name,
  className,
}: {
  label: ReactNode;
  children: ReactNode;
  name: string;
  className?: string;
}) {
  const id = useId();
  const anchor = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const anchorPosition = useRef({ left: 0, bottom: 0 });
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<PositionState>({
    left: 0,
    top: 0,
    placement: "below",
    arrowOffset: 16,
  });
  const pinned = useRef(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  function cancelClose() {
    if (closeTimer.current) clearTimeout(closeTimer.current);
  }
  function show() {
    cancelClose();
    if (anchor.current) {
      const rect = anchor.current.getBoundingClientRect();
      const left = Math.max(8, Math.min(rect.left, window.innerWidth - 300));
      const top = rect.bottom + 8;
      const anchorCenter = rect.left + rect.width / 2;
      const arrowOffset = Math.max(12, Math.min(260, anchorCenter - left - 4));
      setPosition({ left, top, placement: "below", arrowOffset });
    }
    setOpen(true);
  }
  function leave() {
    cancelClose();
    if (!pinned.current) closeTimer.current = setTimeout(() => setOpen(false), 120);
  }
  function close() {
    pinned.current = false;
    cancelClose();
    setOpen(false);
  }
  useLayoutEffect(() => {
    if (!open || !anchor.current || !panel.current) return;
    const rect = anchor.current.getBoundingClientRect();
    anchorPosition.current = { left: rect.left, bottom: rect.bottom };
    const bounds = panel.current.getBoundingClientRect();
    const left = Math.max(8, Math.min(rect.left, window.innerWidth - bounds.width - 8));
    const below = rect.bottom + 8;
    const isBelow = below + bounds.height <= window.innerHeight - 8;
    const top = isBelow ? below : Math.max(8, rect.top - bounds.height - 8);
    const placement: "below" | "above" = isBelow ? "below" : "above";
    const anchorCenter = rect.left + rect.width / 2;
    const arrowOffset = Math.max(12, Math.min(bounds.width - 20, anchorCenter - left - 4));
    setPosition({ left, top, placement, arrowOffset });
  }, [open]);
  useEffect(() => {
    if (!open) return;
    function dismiss(event: Event) {
      if (event.type === "keydown" && (event as KeyboardEvent).key !== "Escape") return;
      if (
        event.type === "pointerdown" &&
        (anchor.current?.contains(event.target as Node) ||
          panel.current?.contains(event.target as Node))
      )
        return;
      if (event.type === "scroll" && panel.current?.contains(event.target as Node)) return;
      if (event.type === "scroll" && anchor.current) {
        const rect = anchor.current.getBoundingClientRect();
        // Focus/click can scroll the trigger into view before opening. Its
        // queued scroll event must not dismiss details already placed there.
        if (
          rect.left === anchorPosition.current.left &&
          rect.bottom === anchorPosition.current.bottom
        )
          return;
      }
      pinned.current = false;
      setOpen(false);
    }
    document.addEventListener("keydown", dismiss);
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", dismiss);
    return () => {
      document.removeEventListener("keydown", dismiss);
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("scroll", dismiss, true);
      window.removeEventListener("resize", dismiss);
    };
  }, [open]);
  useEffect(
    () => () => {
      if (closeTimer.current) clearTimeout(closeTimer.current);
    },
    [],
  );
  return (
    <>
      <button
        ref={anchor}
        type="button"
        className={`${styles.trigger}${className ? ` ${className}` : ""}`}
        aria-label={name}
        aria-describedby={open ? id : undefined}
        onPointerEnter={show}
        onPointerLeave={leave}
        onFocus={show}
        onBlur={close}
        onClick={() => {
          if (pinned.current) close();
          else {
            pinned.current = true;
            show();
          }
        }}
      >
        {label}
      </button>
      {open
        ? createPortal(
            <div
              ref={panel}
              id={id}
              role="tooltip"
              data-placement={position.placement}
              className={styles.panel}
              style={
                {
                  left: position.left,
                  top: position.top,
                  "--arrow-offset": `${position.arrowOffset}px`,
                } as CSSProperties
              }
              onPointerEnter={cancelClose}
              onPointerLeave={leave}
            >
              <div className={styles.arrow} aria-hidden="true" />
              <div className={styles.content}>{children}</div>
            </div>,
            document.body,
          )
        : null}
    </>
  );
}

/** Structured card wrapper for rich entity inspection tooltips. */
export function HoverCard({
  title,
  tags,
  children,
}: {
  title?: ReactNode;
  tags?: ReactNode[];
  children: ReactNode;
}) {
  return (
    <div className={styles.card}>
      {title || (tags && tags.length > 0) ? (
        <>
          <div className={styles.cardHeader}>
            {title ? <span className={styles.cardTitle}>{title}</span> : <span />}
            {tags && tags.length > 0 ? (
              <div className={styles.cardBadges}>
                {tags.map((tag, i) => (
                  <span key={i} className={styles.cardTag}>
                    {tag}
                  </span>
                ))}
              </div>
            ) : null}
          </div>
          <div className={styles.cardDivider} />
        </>
      ) : null}
      {children}
    </div>
  );
}
