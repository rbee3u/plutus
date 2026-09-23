import { Check } from "lucide-react";
import {
  type ButtonHTMLAttributes,
  type CSSProperties,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router";
import { iconSize } from "@/shared/icons/iconSizes";
import styles from "@/shared/ui/Menu.module.css";

export type MenuPlacement = "bottom-end" | "bottom-start" | "top-start";

const ITEM_SELECTOR =
  '[role="menuitem"]:not([disabled]), [role="menuitemradio"]:not([disabled]), [role="option"]:not([disabled])';
const GAP = 4;

/**
 * State for one popover menu: open flag, positioning against its button,
 * light dismiss on outside pointer or Escape, and arrow-key movement.
 */
export function useMenu(placement: MenuPlacement = "bottom-end") {
  const [open, setOpen] = useState(false);
  // Callback refs kept in state: effects re-run when the nodes mount.
  const [anchor, setAnchor] = useState<HTMLButtonElement | null>(null);
  const [panel, setPanel] = useState<HTMLDivElement | null>(null);
  const [style, setStyle] = useState<CSSProperties>({});

  const close = useCallback(
    (restoreFocus = false) => {
      setOpen(false);
      if (restoreFocus) {
        anchor?.focus();
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
      if (placement === "top-start") {
        setStyle({ bottom: window.innerHeight - rect.top + GAP, left: rect.left });
      } else if (placement === "bottom-start") {
        setStyle({ top: rect.bottom + GAP, left: rect.left });
      } else {
        setStyle({ top: rect.bottom + GAP, right: Math.max(8, window.innerWidth - rect.right) });
      }
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [anchor, open, placement]);

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
      )?.focus();
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
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      window.cancelAnimationFrame(frame);
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
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
