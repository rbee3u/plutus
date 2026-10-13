import { Check } from "lucide-react";
import {
  type ButtonHTMLAttributes,
  type CSSProperties,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router";
import { iconSize } from "@/shared/icons/iconSizes";
import styles from "@/shared/ui/Menu.module.css";

export type MenuPlacement = "bottom-end" | "bottom-start" | "top-start" | "top-end";

const ITEM_SELECTOR =
  '[role="menuitem"]:not([disabled]), [role="menuitemradio"]:not([disabled]), [role="option"]:not([disabled])';
const GAP = 4;
const VIEWPORT_PADDING = 8;

/**
 * State for one popover menu: open flag, positioning against its button,
 * light dismiss on outside pointer or Escape, and arrow-key movement.
 */
export function useMenu(placement: MenuPlacement = "bottom-end") {
  const [open, setOpen] = useState(false);
  // Callback refs kept in state: effects re-run when the nodes mount.
  const [anchor, setAnchor] = useState<HTMLButtonElement | null>(null);
  const [panel, setPanelState] = useState<HTMLDivElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const [style, setStyle] = useState<CSSProperties>({});

  const setPanel = useCallback((node: HTMLDivElement | null) => {
    panelRef.current = node;
    setPanelState(node);
  }, []);

  const close = useCallback(
    (restoreFocus = false) => {
      setOpen(false);
      if (restoreFocus) {
        anchor?.focus({ preventScroll: true });
      }
    },
    [anchor],
  );

  useLayoutEffect(() => {
    if (!open || !anchor) {
      return;
    }
    const update = () => {
      const rect = anchor.getBoundingClientRect();
      const currentPanel = panel ?? panelRef.current;
      const panelHeight = currentPanel?.offsetHeight || 200;

      const spaceBelow = window.innerHeight - rect.bottom - GAP - VIEWPORT_PADDING;
      const spaceAbove = rect.top - GAP - VIEWPORT_PADDING;

      let effectivePlacement = placement;
      if (placement.startsWith("bottom") && spaceBelow < panelHeight && spaceAbove > spaceBelow) {
        effectivePlacement = placement === "bottom-end" ? "top-end" : "top-start";
      } else if (
        placement.startsWith("top") &&
        spaceAbove < panelHeight &&
        spaceBelow > spaceAbove
      ) {
        effectivePlacement = placement === "top-end" ? "bottom-end" : "bottom-start";
      }

      const availableHeight = effectivePlacement.startsWith("top") ? spaceAbove : spaceBelow;
      const maxHeight = Math.min(window.innerHeight - 16, Math.max(80, availableHeight));

      if (effectivePlacement === "top-start") {
        setStyle({
          bottom: window.innerHeight - rect.top + GAP,
          left: Math.max(VIEWPORT_PADDING, rect.left),
          maxHeight,
        });
      } else if (effectivePlacement === "top-end") {
        setStyle({
          bottom: window.innerHeight - rect.top + GAP,
          right: Math.max(VIEWPORT_PADDING, window.innerWidth - rect.right),
          maxHeight,
        });
      } else if (effectivePlacement === "bottom-start") {
        setStyle({
          top: rect.bottom + GAP,
          left: Math.max(VIEWPORT_PADDING, rect.left),
          maxHeight,
        });
      } else {
        setStyle({
          top: rect.bottom + GAP,
          right: Math.max(VIEWPORT_PADDING, window.innerWidth - rect.right),
          maxHeight,
        });
      }
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [anchor, open, panel, placement]);

  useEffect(() => {
    if (!open || !panel) {
      return;
    }
    const items = () => [...panel.querySelectorAll<HTMLElement>(ITEM_SELECTOR)];
    const frame = window.requestAnimationFrame(() => {
      const list = items();
      (
        list.find((item) => item.getAttribute("aria-checked") === "true") ??
        list.find((item) => item.getAttribute("aria-selected") === "true") ??
        list[0]
      )?.focus({ preventScroll: true });
    });
    const onPointer = (event: MouseEvent) => {
      const target = event.target as Node;
      if (anchor?.contains(target) || panel.contains(target)) {
        return;
      }
      setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close(true);
        return;
      }
      if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
        return;
      }
      const list = items();
      if (list.length === 0) {
        return;
      }
      const index = list.findIndex((item) => item === document.activeElement);
      const last = list.length - 1;
      const next =
        event.key === "Home"
          ? 0
          : event.key === "End"
            ? last
            : event.key === "ArrowDown"
              ? (index + 1) % list.length
              : index <= 0
                ? last
                : index - 1;
      event.preventDefault();
      list[next]?.focus();
    };
    const onScroll = (event: Event) => {
      if (!panel.contains(event.target as Node)) close();
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    document.addEventListener("scroll", onScroll, true);
    return () => {
      window.cancelAnimationFrame(frame);
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("scroll", onScroll, true);
    };
  }, [anchor, close, open, panel]);

  return { open, setOpen, close, style, setAnchor, setPanel };
}

export function Menu({
  menu,
  role = "menu",
  label,
  labelledBy,
  children,
}: {
  menu: ReturnType<typeof useMenu>;
  role?: "menu" | "listbox";
  label?: string;
  labelledBy?: string;
  children: ReactNode;
}) {
  if (!menu.open) {
    return null;
  }
  return createPortal(
    <div
      ref={(node) => menu.setPanel(node)}
      className={styles.menu}
      role={role}
      aria-label={label}
      aria-labelledby={labelledBy}
      style={menu.style}
    >
      {children}
    </div>,
    document.body,
  );
}

type MenuItemProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "role"> & {
  role?: "menuitem" | "menuitemradio" | "option";
  /** Renders a router link instead of a button. */
  to?: string;
  icon?: ReactNode;
  /** Shows a check mark; also sets aria-checked or aria-selected for the role. */
  checked?: boolean;
  /** Renders the item in the danger colour. */
  danger?: boolean;
};

export function MenuItem({
  role = "menuitem",
  to,
  icon,
  checked,
  danger,
  className,
  children,
  ...props
}: MenuItemProps) {
  const classes = `${styles.item} ${danger ? styles.danger : ""} ${className ?? ""}`;
  const state =
    checked === undefined
      ? {}
      : role === "option"
        ? { "aria-selected": checked }
        : { "aria-checked": checked };
  const body = (
    <>
      {icon ? (
        <span className={styles.icon} aria-hidden="true">
          {icon}
        </span>
      ) : null}
      <span className={styles.label}>{children}</span>
      {checked ? <Check size={iconSize.sm} className={styles.check} aria-hidden="true" /> : null}
    </>
  );
  if (to) {
    return (
      <Link role={role} to={to} className={classes} onClick={props.onClick as never} {...state}>
        {body}
      </Link>
    );
  }
  return (
    <button {...props} {...state} type="button" role={role} className={classes}>
      {body}
    </button>
  );
}

export function MenuSeparator() {
  return <div className={styles.separator} role="separator" />;
}
