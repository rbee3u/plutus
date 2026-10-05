import { Menu as MenuIcon, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Outlet, useLocation, useNavigate, useParams } from "react-router";
import { listPortfolios, postAuthLogout, problemMessage, refreshCsrf } from "@/api";
import { PortfolioSwitcher, type PortfolioOption } from "@/app/shell/PortfolioSwitcher";
import {
  HoldingsNavigationProvider,
  SessionLink,
} from "@/features/holdings/components/HoldingsNavigation";
import { SidebarUtilities } from "@/app/shell/SidebarUtilities";
import { UserMenu } from "@/app/shell/UserMenu";
import {
  LAST_PORTFOLIO_KEY,
  isAccountPath,
  isNavItemActive,
  navigationFor,
  pageHeading,
} from "@/app/shell/navigation";
import { useMobileNavigation } from "@/app/shell/useMobileNavigation";
import { usePersistentTheme } from "@/app/theme/usePersistentTheme";
import { readPreference, storePreference } from "@/shared/lib/preferences";
import { iconSize } from "@/shared/icons/iconSizes";
import { PlutusIcon } from "@/shared/icons/PlutusIcon";
import { IconButton } from "@/shared/ui/IconButton";
import styles from "@/app/shell/Shell.module.css";

export function Shell() {
  return (
    <HoldingsNavigationProvider>
      <ShellContent />
    </HoldingsNavigationProvider>
  );
}

function ShellContent() {
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const urlPortfolioId = params.portfolioId ?? "";
  const [rememberedId, setRememberedId] = useState(() => readPreference(LAST_PORTFOLIO_KEY) || "");
  const navPortfolioId = urlPortfolioId || rememberedId;
  const [collapsed, setCollapsed] = useState(
    () => readPreference("plutus-sidebar-collapsed") === "true",
  );
  const [theme, setTheme] = usePersistentTheme();
  const [portfolios, setPortfolios] = useState<PortfolioOption[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const { open, setOpen, close, mobileLayout, sidebarRef, menuButtonRef } = useMobileNavigation();
  const groups = navPortfolioId ? navigationFor(navPortfolioId) : [];
  const heading = pageHeading(location.pathname, location.search);

  useEffect(() => {
    document.title = `${heading.documentTitle} · Plutus`;
  }, [heading.documentTitle]);

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
        const items = result.data.items.map((item) => ({ id: item.id, name: item.name }));
        setPortfolios(items);
        if (urlPortfolioId) {
          if (!items.some((item) => item.id === urlPortfolioId)) {
            const remembered = readPreference(LAST_PORTFOLIO_KEY);
            const next = items.find((item) => item.id === remembered) ?? items[0];
            if (next) {
              void navigate(`/p/${next.id}/overview`, { replace: true });
            }
          } else {
            storePreference(LAST_PORTFOLIO_KEY, urlPortfolioId);
            setRememberedId(urlPortfolioId);
          }
        } else if (isAccountPath(location.pathname)) {
          const remembered = readPreference(LAST_PORTFOLIO_KEY);
          const next = items.find((item) => item.id === remembered) ?? items[0];
          if (next) {
            storePreference(LAST_PORTFOLIO_KEY, next.id);
            setRememberedId(next.id);
          }
        }
      } catch (error) {
        setLoadError(
          problemMessage(error, "Portfolios could not be loaded. Check the network and retry."),
        );
      }
    })();
  }, [location.pathname, navigate, urlPortfolioId]);

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
        <div className={styles.brandRow}>
          <SessionLink className={styles.brand} to="/" title={collapsed ? "Plutus" : undefined}>
            <PlutusIcon size={iconSize.lg} />
            <strong>Plutus</strong>
          </SessionLink>
          {mobileLayout ? (
            <IconButton
              className={styles.drawerCloseButton}
              label="Close navigation"
              onClick={() => close()}
            >
              <X size={iconSize.md} aria-hidden="true" />
            </IconButton>
          ) : null}
        </div>
        <nav className={styles.moduleNav} aria-label="Modules">
          {groups.map((group) => (
            <div key={group.id} className={styles.navGroup}>
              {group.label ? <span className={styles.navGroupLabel}>{group.label}</span> : null}
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <SessionLink
                    key={item.to}
                    to={item.to}
                    title={collapsed ? item.label : undefined}
                    aria-current={isNavItemActive(location.pathname, item) ? "page" : undefined}
                    onClick={() => {
                      if (mobileLayout) close(false);
                    }}
                  >
                    <Icon size={iconSize.sm} aria-hidden="true" />
                    <span>{item.label}</span>
                  </SessionLink>
                );
              })}
            </div>
          ))}
        </nav>
        <SidebarUtilities
          collapsed={collapsed}
          mobileLayout={mobileLayout}
          onThemeChange={setTheme}
          onToggleCollapsed={() => setCollapsed((value) => !value)}
          theme={theme}
          onSignOut={() => void logout()}
        />
      </aside>
      {open && <div className={styles.scrim} aria-hidden="true" onClick={() => close()} />}
      <div className={styles.workspace}>
        <header className={styles.topbar}>
          {mobileLayout ? (
            <IconButton
              className={styles.menuButton}
              label="Open navigation"
              aria-controls="console-navigation"
              aria-expanded={open}
              onClick={() => setOpen(true)}
              ref={menuButtonRef}
            >
              <MenuIcon size={iconSize.md} aria-hidden="true" />
            </IconButton>
          ) : null}
          <h1 id="console-page-title" className={styles.pageTitle}>
            {heading.title}
          </h1>
          {heading.tabs.length > 0 ? (
            <nav className={styles.tabs} aria-label={`${heading.title} views`}>
              {heading.tabs.map((tab) => (
                <SessionLink
                  key={tab.label}
                  to={tab.to}
                  aria-current={tab.active ? "page" : undefined}
                >
                  {tab.label}
                </SessionLink>
              ))}
            </nav>
          ) : null}
          <div className={styles.topbarEnd}>
            <PortfolioSwitcher
              portfolioId={navPortfolioId}
              portfolios={portfolios}
              onPortfolioRemembered={setRememberedId}
            />
            <UserMenu onSignOut={() => void logout()} />
          </div>
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
            // Page state (notices, open rows, a followed cycle) belongs to one Portfolio.
            <Outlet key={urlPortfolioId} />
          )}
        </main>
      </div>
    </div>
  );
}

export function RootRedirect() {
  return (
    <div className={styles.boot}>
      <PlutusIcon size={iconSize.xl} />
      <span>Opening Plutus</span>
    </div>
  );
}
