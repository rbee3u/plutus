import { ChevronDown } from "lucide-react";
import { type CSSProperties, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router";
import { useCurrentUser } from "@/shared/lib/session";
import { iconSize } from "@/shared/icons/iconSizes";
import styles from "@/app/shell/Shell.module.css";

export function UserMenu({ onSignOut }: { onSignOut: () => void }) {
  const user = useCurrentUser();
  const [open, setOpen] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [menuStyle, setMenuStyle] = useState<CSSProperties>({});
  const username = user.data?.username ?? "Account";

  useLayoutEffect(() => {
    if (!open || !buttonRef.current) {
      return;
    }
    const update = () => {
      const rect = buttonRef.current?.getBoundingClientRect();
      if (!rect) {
        return;
      }
      setMenuStyle({
        top: rect.bottom + 4,
        right: Math.max(8, window.innerWidth - rect.right),
      });
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const onPointer = (event: MouseEvent) => {
      const target = event.target as Node;
      if (buttonRef.current?.contains(target) || menuRef.current?.contains(target)) {
        return;
      }
      setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        className={styles.userMenuButton}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <span className={styles.switcherName}>{username}</span>
        <ChevronDown size={iconSize.xs} aria-hidden="true" />
      </button>
      {open
        ? createPortal(
            <div ref={menuRef} className={styles.userMenu} role="menu" style={menuStyle}>
              <Link
                role="menuitem"
                className={styles.userMenuItem}
                to="/account/security"
                onClick={() => setOpen(false)}
              >
                Account & Security
              </Link>
              <button
                type="button"
                role="menuitem"
                className={styles.userMenuItem}
                onClick={() => {
                  setOpen(false);
                  onSignOut();
                }}
              >
                Sign out
              </button>
            </div>,
            document.body,
          )
        : null}
    </>
  );
}
