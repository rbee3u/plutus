import { ChevronLeft, ChevronRight, Monitor, Moon, Sun } from "lucide-react";
import type { ThemePreference } from "@/app/theme/usePersistentTheme";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import { Menu, MenuItem, useMenu } from "@/shared/ui/Menu";
import styles from "@/app/shell/Shell.module.css";

const themeOptions = [
  { icon: Monitor, label: "System", value: "system" as const },
  { icon: Sun, label: "Light", value: "light" as const },
  { icon: Moon, label: "Dark", value: "dark" as const },
];

interface SidebarUtilitiesProps {
  collapsed: boolean;
  mobileLayout: boolean;
  onThemeChange: (theme: ThemePreference) => void;
  onToggleCollapsed: () => void;
  theme: ThemePreference;
}

export function SidebarUtilities({
  collapsed,
  mobileLayout,
  onThemeChange,
  onToggleCollapsed,
  theme,
}: SidebarUtilitiesProps) {
  return (
    <footer className={styles.sidebarUtilities}>
      <ThemeMenu collapsed={collapsed} onChange={onThemeChange} value={theme} />
      <div className={styles.sidebarMeta}>
        <span className={styles.sidebarVersion}>v0.1.0-dev</span>
        {mobileLayout ? null : (
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
        )}
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
  const menu = useMenu("top-start");
  const current = themeOptions.find((option) => option.value === value) ?? themeOptions[0];
  const Current = current?.icon ?? Monitor;

  return (
    <>
      <button
        ref={(node) => menu.setAnchor(node)}
        type="button"
        className={styles.utilityItem}
        title={collapsed ? `Theme: ${current?.label ?? "System"}` : undefined}
        aria-haspopup="menu"
        aria-expanded={menu.open}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <Current size={iconSize.sm} aria-hidden="true" />
        <span className={styles.utilityLabel}>Theme</span>
        <span className={styles.utilityValue}>{current?.label}</span>
      </button>
      <Menu menu={menu} label="Theme">
        {themeOptions.map((option) => {
          const Icon = option.icon;
          return (
            <MenuItem
              key={option.value}
              role="menuitemradio"
              icon={<Icon size={iconSize.sm} />}
              checked={option.value === value}
              onClick={() => {
                onChange(option.value);
                menu.close(true);
              }}
            >
              {option.label}
            </MenuItem>
          );
        })}
      </Menu>
    </>
  );
}
