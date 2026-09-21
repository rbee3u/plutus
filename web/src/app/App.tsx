import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes, useNavigate } from "react-router";
import { getAuthMe, listPortfolios } from "@/api/generated/sdk.gen";
import { configureApiClient, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY } from "@/app/shell/navigation";
import { ModuleSubnav } from "@/app/shell/RoutedSubnav";
import { RootRedirect, Shell } from "@/app/shell/Shell";
import { LoginPage } from "@/features/auth/LoginPage";
import { BaselinePage } from "@/features/capital/BaselinePage";
import { FlowsPage } from "@/features/capital/FlowsPage";
import { OverviewPage } from "@/features/overview/OverviewPage";
import { AuditPage } from "@/features/data/AuditPage";
import { CatalogPage } from "@/features/data/CatalogPage";
import { ConnectionsPage } from "@/features/data/ConnectionsPage";
import { IdentitiesPage } from "@/features/data/IdentitiesPage";
import { JobsPage } from "@/features/data/JobsPage";
import { ObservationsPage } from "@/features/data/ObservationsPage";
import { ExposurePage } from "@/features/portfolio/ExposurePage";
import { BalancesPage } from "@/features/portfolio/BalancesPage";
import { PositionsPage } from "@/features/portfolio/PositionsPage";
import { ValuationsPage } from "@/features/history/ValuationsPage";
import { ValueChangesPage } from "@/features/history/ValueChangesPage";
import { PortfolioSettingsPage } from "@/features/settings/PortfolioSettingsPage";
import { SecuritySettingsPage } from "@/features/settings/SecuritySettingsPage";
import { readPreference, storePreference } from "@/shared/lib/preferences";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

configureApiClient();

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<Bootstrap />} />
          <Route path="/p/:portfolioId" element={<Shell />}>
            <Route path="overview" element={<OverviewPage />} />
            <Route path="portfolio/balances" element={<BalancesPage />} />
            <Route path="portfolio/positions" element={<PositionsPage />} />
            <Route path="portfolio/exposure" element={<ExposurePage />} />
            <Route element={<ModuleSubnav moduleId="history" />}>
              <Route path="history/valuations" element={<ValuationsPage />} />
              <Route path="history/value-changes" element={<ValueChangesPage />} />
            </Route>
            <Route element={<ModuleSubnav moduleId="capital" />}>
              <Route path="capital/baseline" element={<BaselinePage />} />
              <Route path="capital/flows" element={<FlowsPage />} />
            </Route>
            <Route element={<ModuleSubnav moduleId="data-sources" />}>
              <Route path="data/identities" element={<IdentitiesPage />} />
              <Route path="data/connections" element={<ConnectionsPage />} />
            </Route>
            <Route element={<ModuleSubnav moduleId="asset-catalog" />}>
              <Route path="data/catalog" element={<CatalogPage />} />
            </Route>
            <Route element={<ModuleSubnav moduleId="data-activity" />}>
              <Route path="data/observations" element={<ObservationsPage />} />
              <Route path="data/jobs" element={<JobsPage />} />
              <Route path="data/audit" element={<AuditPage />} />
            </Route>
            <Route path="settings/portfolio" element={<PortfolioSettingsPage />} />
            <Route path="settings/security" element={<SecuritySettingsPage />} />
            <Route index element={<Navigate to="overview" replace />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function Bootstrap() {
  const navigate = useNavigate();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        await refreshCsrf();
        const me = await getAuthMe();
        if (me.response?.status === 401 || me.error || !me.data) {
          void navigate("/login", { replace: true });
          return;
        }
        const listed = await listPortfolios();
        const items = listed.data?.items.filter((item) => item.status === "active") ?? [];
        const remembered = readPreference(LAST_PORTFOLIO_KEY);
        const next = items.find((item) => item.id === remembered) ?? items[0];
        if (!next) {
          setFailed(true);
          return;
        }
        storePreference(LAST_PORTFOLIO_KEY, next.id);
        void navigate(`/p/${next.id}/overview`, { replace: true });
      } catch {
        setFailed(true);
      }
    })();
  }, [navigate]);

  if (failed) {
    return (
      <p>
        Plutus could not restore the session. <a href="/login">Sign in</a>
      </p>
    );
  }

  return <RootRedirect />;
}
