import { Box, Menu, X } from "lucide-react";
import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate, useParams } from "react-router";
import { listPortfolios, postAuthLogout } from "@/api/generated/sdk.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { PortfolioSwitcher, type PortfolioOption } from "@/app/shell/PortfolioSwitcher";
import { BrandMark, SidebarUtilities } from "@/app/shell/SidebarUtilities";
import { LAST_PORTFOLIO_KEY, navigationFor, pageTitle } from "@/app/shell/navigation";
import { useMobileNavigation } from "@/app/shell/useMobileNavigation";
import { usePersistentTheme } from "@/app/theme/usePersistentTheme";
import { readPreference, storePreference } from "@/shared/lib/preferences";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import styles from "@/app/shell/Shell.module.css";

export function Shell() {
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const portfolioId = params.portfolioId ?? "";
  const [collapsed, setCollapsed] = useState(
    () => readPreference("plutus-sidebar-collapsed") === "true",
  );
  const [theme, setTheme] = usePersistentTheme();
  const [portfolios, setPortfolios] = useState<PortfolioOption[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const { open, setOpen, close, mobileLayout, sidebarRef, menuButtonRef } = useMobileNavigation();
  const groups = navigationFor(portfolioId);
  const title = pageTitle(location.pathname);

  useEffect(() => {
    document.title = `${title} - Plutus`;
  }, [title]);

  useEffect(() => {
    storePreference("plutus-sidebar-collapsed", String(collapsed));
  }, [collapsed]);

  useEffect(() => {
    void (async () => {
      try {
        const result = await listPortfolios();
        if (result.response?.status === 401) {
          void navigate("/login", { replace: true });
          return;
        }
        if (result.error || !result.data) {
          setLoadError(problemMessage(result.error, "Portfolios could not be loaded."));
          return;
        }
        const items = result.data.items
          .filter((item) => item.status === "active")
          .map((item) => ({ id: item.id, name: item.name, undeletable: item.undeletable }));
        setPortfolios(items);
        if (!items.some((item) => item.id === portfolioId)) {
          const remembered = readPreference(LAST_PORTFOLIO_KEY);
          const next = items.find((item) => item.id === remembered) ?? items[0];
          if (next) {
            void navigate(`/p/${next.id}/overview`, { replace: true });
          }
        } else {
          storePreference(LAST_PORTFOLIO_KEY, portfolioId);
        }
      } catch (error) {
        setLoadError(
          problemMessage(error, "Portfolios could not be loaded. Check the network and retry."),
        );
      }
    })();
  }, [navigate, portfolioId]);

  async function logout() {
    try {
      await refreshCsrf();
      await postAuthLogout();
    } finally {
      void navigate("/login", { replace: true });
    }
  }

  return (
    <div className={`${styles.app} ${collapsed ? styles.collapsed : ""}`} data-plutus-shell="true">
      <a className={styles.skipLink} href="#main-content">
        Skip to main content
      </a>
      <aside
        ref={sidebarRef}
        id="console-navigation"
        className={`${styles.sidebar} ${open ? styles.mobileOpen : ""}`}
        aria-label="Application navigation"
        aria-hidden={mobileLayout && !open ? "true" : undefined}
      >
        <div className={styles.brand} title={collapsed ? "Plutus" : undefined}>
          <BrandMark />
          <span className={styles.brandCopy}>
            <strong>Plutus</strong>
            <span className={styles.brandSeparator} aria-hidden="true">
              ·
            </span>
            <small>Observe portfolios</small>
          </span>
          <IconButton
            className={styles.drawerCloseButton}
            label="Close navigation"
            onClick={() => close()}
          >
            <X size={iconSize.md} aria-hidden="true" />
          </IconButton>
        </div>
        <nav className={styles.moduleNav} aria-label="Modules">
          {groups.map((group) => (
            <div key={group.id} className={styles.navGroup}>
              {group.label ? <span className={styles.navGroupLabel}>{group.label}</span> : null}
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    title={collapsed ? item.label : undefined}
                    onClick={() => {
                      if (mobileLayout) close(false);
                    }}
                  >
                    <Icon size={iconSize.md} />
                    <strong>{item.label}</strong>
                  </NavLink>
                );
              })}
            </div>
          ))}
        </nav>
        <SidebarUtilities
          collapsed={collapsed}
          onThemeChange={setTheme}
          onToggleCollapsed={() => setCollapsed((value) => !value)}
          theme={theme}
        />
      </aside>
      {open && <div className={styles.scrim} aria-hidden="true" onClick={() => close()} />}
      <div className={styles.workspace}>
        <header className={styles.topbar}>
          <IconButton
            className={styles.menuButton}
            label="Open navigation"
            aria-controls="console-navigation"
            aria-expanded={open}
            onClick={() => setOpen(true)}
            ref={menuButtonRef}
          >
            <Menu size={iconSize.md} />
          </IconButton>
          <h1 id="console-page-title" className={styles.pageTitle}>
            {title}
          </h1>
          <PortfolioSwitcher portfolioId={portfolioId} portfolios={portfolios} />
          <button type="button" onClick={() => void logout()}>
            Sign out
          </button>
        </header>
        <main
          id="main-content"
          className={styles.content}
          aria-labelledby="console-page-title"
          tabIndex={-1}
        >
          {loadError ? (
            <div className={styles.startupError} role="alert">
              <p>{loadError}</p>
              <button type="button" onClick={() => window.location.reload()}>
                Retry
              </button>
            </div>
          ) : (
            <Outlet />
          )}
        </main>
      </div>
    </div>
  );
}

export function RootRedirect() {
  return (
    <div className={styles.boot}>
      <Box size={iconSize.xl} />
      <span>Opening Plutus</span>
    </div>
  );
}
