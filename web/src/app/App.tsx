import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes, useNavigate, useParams } from "react-router";
import { getAuthMe, listPortfolios } from "@/api/generated/sdk.gen";
import { configureApiClient, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY } from "@/app/shell/navigation";
import { RootRedirect, Shell } from "@/app/shell/Shell";
import { LoginPage } from "@/features/auth/LoginPage";
import { CapitalPage } from "@/features/capital/CapitalPage";
import { ActivityPage } from "@/features/data/ActivityPage";
import { CatalogPage } from "@/features/data/CatalogPage";
import { IdentitiesPage } from "@/features/data/IdentitiesPage";
import { ValuationPointsPage } from "@/features/history/ValuationPointsPage";
import { ValueChangesPage } from "@/features/history/ValueChangesPage";
import { BalancesPage } from "@/features/holdings/BalancesPage";
import { PositionsPage } from "@/features/holdings/PositionsPage";
import { OverviewPage } from "@/features/overview/OverviewPage";
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

/** Old bookmarks from the projection-era information architecture. */
const LEGACY_PATHS: readonly { from: string; to: string }[] = [
  { from: "history/valuations", to: "history" },
  { from: "capital/baseline", to: "capital" },
  { from: "capital/flows", to: "capital" },
  { from: "data/observations", to: "data/activity?tab=cycles" },
  { from: "data/jobs", to: "data/activity?tab=jobs" },
  { from: "data/audit", to: "data/activity?tab=audit" },
];

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<Bootstrap />} />
          <Route element={<Shell />}>
            <Route path="account/security" element={<SecuritySettingsPage />} />
            <Route path="p/:portfolioId">
              <Route path="overview" element={<OverviewPage />} />
              <Route path="holdings" element={<Navigate to="balances" replace />} />
              <Route path="holdings/balances" element={<BalancesPage />} />
              <Route path="holdings/futures" element={<PositionsPage />} />
              <Route path="holdings/dual-investment" element={<PositionsPage key="dual" dual />} />
              <Route path="history" element={<ValuationPointsPage />} />
              <Route path="history/value-changes" element={<ValueChangesPage />} />
              <Route path="capital" element={<CapitalPage />} />
              <Route path="data" element={<Navigate to="identities" replace />} />
              <Route path="data/identities" element={<IdentitiesPage />} />
              <Route path="data/catalog" element={<CatalogPage />} />
              <Route path="data/activity" element={<ActivityPage />} />
              <Route path="settings" element={<Navigate to="portfolio" replace />} />
              <Route path="settings/portfolio" element={<PortfolioSettingsPage />} />
              <Route path="settings/security" element={<LegacySecurityRedirect />} />
              {LEGACY_PATHS.map((item) => (
                <Route
                  key={item.from}
                  path={item.from}
                  element={<PortfolioRedirect to={item.to} />}
                />
              ))}
              <Route index element={<Navigate to="overview" replace />} />
            </Route>
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
        const items = listed.data?.items ?? [];
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

function LegacySecurityRedirect() {
  const { portfolioId } = useParams();
  if (portfolioId) {
    storePreference(LAST_PORTFOLIO_KEY, portfolioId);
  }
  return <Navigate to="/account/security" replace />;
}

function PortfolioRedirect({ to }: { to: string }) {
  const { portfolioId = "" } = useParams();
  return <Navigate to={`/p/${portfolioId}/${to}`} replace />;
}
