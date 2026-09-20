import { Box, Check, ChevronLeft, ChevronRight, ChevronUp, Monitor, Moon, Sun } from "lucide-react";
import type { CSSProperties } from "react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { ThemePreference } from "@/app/theme/usePersistentTheme";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import styles from "@/app/shell/Shell.module.css";

const themeOptions = [
  { icon: Monitor, label: "System", value: "system" as const },
  { icon: Sun, label: "Light", value: "light" as const },
  { icon: Moon, label: "Dark", value: "dark" as const },
];

interface SidebarUtilitiesProps {
  collapsed: boolean;
  onThemeChange: (theme: ThemePreference) => void;
  onToggleCollapsed: () => void;
  theme: ThemePreference;
}

export function SidebarUtilities({
  collapsed,
  onThemeChange,
  onToggleCollapsed,
  theme,
}: SidebarUtilitiesProps) {
  return (
    <footer className={styles.sidebarUtilities}>
      <ThemeMenu collapsed={collapsed} onChange={onThemeChange} value={theme} />
      <div className={styles.sidebarMeta}>
        <span className={styles.sidebarVersion}>dev</span>
        <IconButton
          className={styles.collapseButton}
          label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          onClick={onToggleCollapsed}
        >
          {collapsed ? (
            <ChevronRight size={iconSize.md} aria-hidden="true" />
          ) : (
            <ChevronLeft size={iconSize.md} aria-hidden="true" />
          )}
        </IconButton>
      </div>
    </footer>
  );
}

function ThemeMenu({
  collapsed,
  onChange,
  value,
}: {
  collapsed: boolean;
  onChange: (theme: ThemePreference) => void;
  value: ThemePreference;
}) {
  const [open, setOpen] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [menuStyle, setMenuStyle] = useState<CSSProperties>({});
  const Current = themeOptions.find((option) => option.value === value)?.icon ?? Monitor;

  useLayoutEffect(() => {
    if (!open || !buttonRef.current) return;
    const rect = buttonRef.current.getBoundingClientRect();
    setMenuStyle({
      top: Math.max(8, rect.top - 8),
      left: rect.right + 8,
      transform: "translateY(-100%)",
    });
  }, [open, collapsed]);

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: MouseEvent) => {
      if (
        menuRef.current?.contains(event.target as Node) ||
        buttonRef.current?.contains(event.target as Node)
      ) {
        return;
      }
      setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
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
        className={`${styles.utilityItem} ${styles.themeButton}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        <Current size={iconSize.md} aria-hidden="true" />
        <span className={styles.utilityLabel}>Theme</span>
        <span className={styles.themeValue}>{value}</span>
        <ChevronUp
          size={iconSize.sm}
          className={`${styles.themeChevron} ${open ? styles.themeChevronOpen : ""}`}
          aria-hidden="true"
        />
      </button>
      {open &&
        createPortal(
          <div ref={menuRef} className={styles.themeMenu} role="menu" style={menuStyle}>
            {themeOptions.map((option) => {
              const Icon = option.icon;
              const checked = option.value === value;
              return (
                <button
                  key={option.value}
                  type="button"
                  role="menuitemradio"
                  aria-checked={checked}
                  className={styles.themeOption}
                  onClick={() => {
                    onChange(option.value);
                    setOpen(false);
                  }}
                >
                  <Icon size={iconSize.md} aria-hidden="true" />
                  {option.label}
                  {checked ? <Check size={iconSize.sm} className={styles.themeCheck} /> : null}
                </button>
              );
            })}
          </div>,
          document.body,
        )}
    </>
  );
}

export function BrandMark() {
  return (
    <span className={styles.mark}>
      <Box size={iconSize.lg} />
    </span>
  );
}
