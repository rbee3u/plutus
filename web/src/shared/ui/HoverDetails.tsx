import {
  useCallback,
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

export interface PositionState {
  left: number;
  top: number;
  placement: "below" | "above" | "right";
  arrowOffset: number;
  arrowOffsetY?: number;
  ready?: boolean;
}

export function computePosition(
  anchorRect: {
    left: number;
    top: number;
    right: number;
    bottom: number;
    width: number;
    height: number;
  },
  panelWidth: number,
  panelHeight: number,
  preferred: "above" | "below" | "right" = "above",
  windowWidth: number = typeof window !== "undefined" ? window.innerWidth : 1024,
  windowHeight: number = typeof window !== "undefined" ? window.innerHeight : 768,
): PositionState {
  const canFitAbove = anchorRect.top - 8 - panelHeight >= 8;
  const canFitBelow = anchorRect.bottom + 8 + panelHeight <= windowHeight - 8;
  const canFitRight = anchorRect.right + 8 + panelWidth <= windowWidth - 8;

  let placement: "above" | "below" | "right" = preferred;

  if (preferred === "right") {
    if (canFitRight) {
      placement = "right";
    } else if (canFitAbove) {
      placement = "above";
    } else {
      placement = "below";
    }
  } else if (preferred === "above") {
    if (canFitAbove) {
      placement = "above";
    } else {
      placement = "below";
    }
  } else {
    if (canFitBelow) {
      placement = "below";
    } else {
      placement = "above";
    }
  }

  if (placement === "right") {
    const left = anchorRect.right + 8;
    const idealTop = anchorRect.top - 8;
    const top = Math.max(8, Math.min(idealTop, windowHeight - panelHeight - 8));
    const anchorCenterY = anchorRect.top + anchorRect.height / 2;
    const arrowOffsetY = Math.max(12, Math.min(panelHeight - 20, anchorCenterY - top - 4));
    return {
      left,
      top,
      placement: "right",
      arrowOffset: 16,
      arrowOffsetY,
    };
  }

  const left = Math.max(8, Math.min(anchorRect.left, windowWidth - panelWidth - 8));
  const top =
    placement === "above" ? Math.max(8, anchorRect.top - panelHeight - 8) : anchorRect.bottom + 8;
  const anchorCenterX = anchorRect.left + anchorRect.width / 2;
  const arrowOffset = Math.max(12, Math.min(panelWidth - 20, anchorCenterX - left - 4));

  return {
    left,
    top,
    placement,
    arrowOffset,
  };
}

/** Rich details available to pointer, keyboard, and touch users outside scroll clips. */
export function HoverDetails({
  label,
  children,
  name,
  className,
  preferredPlacement = "above",
}: {
  label: ReactNode;
  children: ReactNode;
  name: string;
  className?: string;
  preferredPlacement?: "above" | "below" | "right";
}) {
  const id = useId();
  const anchor = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const anchorPosition = useRef({ left: 0, bottom: 0 });
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<PositionState>(() => ({
    left: 0,
    top: 0,
    placement: preferredPlacement,
    arrowOffset: 16,
  }));
  const pinned = useRef(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const openTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  function cancelTimers() {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
    if (openTimer.current) {
      clearTimeout(openTimer.current);
      openTimer.current = null;
    }
  }

  const updatePosition = useCallback(() => {
    if (!anchor.current || !panel.current) return;
    const rect = anchor.current.getBoundingClientRect();
    anchorPosition.current = { left: rect.left, bottom: rect.bottom };
    const bounds = panel.current.getBoundingClientRect();
    const measured = computePosition(
      rect,
      bounds.width,
      bounds.height,
      preferredPlacement,
      window.innerWidth,
      window.innerHeight,
    );
    setPosition({ ...measured, ready: true });
  }, [preferredPlacement]);

  const showImmediate = useCallback(() => {
    cancelTimers();
    if (!open) {
      setOpen(true);
    } else {
      updatePosition();
    }
  }, [open, updatePosition]);

  function handlePointerEnter() {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
    if (open) return;
    if (openTimer.current) clearTimeout(openTimer.current);
    openTimer.current = setTimeout(() => {
      showImmediate();
    }, 120);
  }

  function handlePointerLeave() {
    if (openTimer.current) {
      clearTimeout(openTimer.current);
      openTimer.current = null;
    }
    if (!pinned.current) {
      if (closeTimer.current) clearTimeout(closeTimer.current);
      closeTimer.current = setTimeout(() => setOpen(false), 120);
    }
  }

  const close = useCallback(() => {
    pinned.current = false;
    cancelTimers();
    setOpen(false);
    setPosition((prev) => (prev.ready ? { ...prev, ready: false } : prev));
  }, []);

  useLayoutEffect(() => {
    if (!open) return;
    updatePosition();
  }, [open, updatePosition, children]);

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
      close();
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
  }, [open, close]);

  useEffect(
    () => () => {
      cancelTimers();
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
        onPointerEnter={handlePointerEnter}
        onPointerLeave={handlePointerLeave}
        onFocus={showImmediate}
        onBlur={close}
        onClick={() => {
          if (pinned.current) close();
          else {
            pinned.current = true;
            showImmediate();
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
                  visibility: position.ready ? undefined : "hidden",
                  "--arrow-offset": `${position.arrowOffset}px`,
                  "--arrow-offset-y": `${position.arrowOffsetY ?? 16}px`,
                } as CSSProperties
              }
              onPointerEnter={cancelTimers}
              onPointerLeave={handlePointerLeave}
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
