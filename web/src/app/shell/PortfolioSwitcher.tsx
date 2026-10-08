import { Briefcase, ChevronDown, Plus } from "lucide-react";
import { useState } from "react";
import { CreatePortfolioDialog } from "@/features/settings/portfolio";
import { useLocation } from "react-router";
import { useSessionNavigation } from "@/shared/navigation/SessionNavigation";
import type { Portfolio } from "@/api";
import { isAccountPath } from "@/app/shell/navigation";
import { LAST_PORTFOLIO_KEY } from "@/shared/lib/preferences";
import { iconSize } from "@/shared/icons/iconSizes";
import { storePreference } from "@/shared/lib/preferences";
import { Menu, MenuItem, MenuSeparator, useMenu } from "@/shared/ui/Menu";
import styles from "@/app/shell/Shell.module.css";

export type PortfolioOption = Pick<Portfolio, "id" | "name">;

export function PortfolioSwitcher({
  portfolioId,
  portfolios,
  onPortfolioRemembered,
}: {
  portfolioId: string;
  portfolios: PortfolioOption[];
  onPortfolioRemembered?: (id: string) => void;
}) {
  const holdingsNavigation = useSessionNavigation();
  const location = useLocation();
  const menu = useMenu();
  const [createOpen, setCreateOpen] = useState(false);
  const current = portfolios.find((item) => item.id === portfolioId) ?? portfolios[0];

  function pathFor(id: string): string {
    if (isAccountPath(location.pathname)) {
      return `/p/${id}/overview`;
    }
    const suffix = location.pathname.replace(/^\/p\/[^/]+/, "") || "/overview";
    return `/p/${id}${suffix}`;
  }

  function selectPortfolio(next: string) {
    storePreference(LAST_PORTFOLIO_KEY, next);
    menu.close(true);
    onPortfolioRemembered?.(next);
    if (isAccountPath(location.pathname)) {
      return;
    }
    holdingsNavigation.go(pathFor(next));
  }

  return (
    <>
      <button
        ref={(node) => menu.setAnchor(node)}
        type="button"
        className={styles.switcherButton}
        title="Switch portfolio"
        aria-haspopup="menu"
        aria-expanded={menu.open}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <Briefcase size={iconSize.xs} aria-hidden="true" />
        <span className={styles.switcherName}>{current?.name ?? "Portfolio"}</span>
        <ChevronDown size={iconSize.xs} aria-hidden="true" />
      </button>
      <Menu menu={menu} label="Portfolios">
        {portfolios.map((item) => (
          <MenuItem
            key={item.id}
            role="menuitemradio"
            checked={item.id === current?.id}
            onClick={() => selectPortfolio(item.id)}
          >
            {item.name}
          </MenuItem>
        ))}
        <MenuSeparator />
        <MenuItem
          icon={<Plus size={iconSize.sm} />}
          onClick={() => {
            menu.close(true);
            setCreateOpen(true);
          }}
        >
          New portfolio
        </MenuItem>
      </Menu>
      <CreatePortfolioDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(portfolio) => {
          storePreference(LAST_PORTFOLIO_KEY, portfolio.id);
          onPortfolioRemembered?.(portfolio.id);
          holdingsNavigation.go(pathFor(portfolio.id));
        }}
      />
    </>
  );
}
