import { RequireSession } from "@/app/RequireSession";
import { usePortfolios } from "@/shared/lib/usePortfolios";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect } from "react";
import { BrowserRouter, Navigate, Route, Routes, useNavigate, useParams } from "react-router";
import { configureApiClient } from "@/api";
import { LAST_PORTFOLIO_KEY } from "@/shared/lib/preferences";
import { RootRedirect, Shell } from "@/app/shell/Shell";
import { LoginPage } from "@/features/auth/pages/LoginPage";
import { CapitalPage } from "@/features/capital/pages/CapitalPage";
import { ActivityPage } from "@/features/manage/pages/ActivityPage";
import { CatalogPage } from "@/features/manage/pages/CatalogPage";
import { IdentitiesPage } from "@/features/manage/pages/IdentitiesPage";
import { ValuationPointsPage } from "@/features/history/pages/ValuationPointsPage";
import { ValueChangesPage } from "@/features/history/pages/ValueChangesPage";
import { BalancesPage } from "@/features/holdings/pages/BalancesPage";
import { EarnPositionsPage } from "@/features/holdings/pages/EarnPositionsPage";
import { DualPositionsPage } from "@/features/holdings/pages/DualPositionsPage";
import { PositionsPage } from "@/features/holdings/pages/PositionsPage";
import { OverviewPage } from "@/features/overview/pages/OverviewPage";
import { PortfolioSettingsPage } from "@/features/settings/pages/PortfolioSettingsPage";
import { SecuritySettingsPage } from "@/features/settings/pages/SecuritySettingsPage";
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
  { from: "data/jobs", to: "data/activity" },
  { from: "data/audit", to: "data/activity?tab=audit" },
];

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<RequireSession />}>
            <Route path="/" element={<Bootstrap />} />
            <Route element={<Shell />}>
              <Route path="account/security" element={<SecuritySettingsPage />} />
              <Route path="p/:portfolioId">
                <Route path="overview" element={<OverviewPage />} />
                <Route path="holdings" element={<Navigate to="balances" replace />} />
                <Route path="holdings/balances" element={<BalancesPage />} />
                <Route path="holdings/futures" element={<PositionsPage />} />
                <Route path="holdings/earn" element={<EarnPositionsPage />} />
                <Route path="holdings/dual-investment" element={<DualPositionsPage />} />
                <Route path="history" element={<ValuationPointsPage />} />
                <Route path="history/value-changes" element={<ValueChangesPage />} />
                <Route path="capital" element={<CapitalPage />} />
                <Route path="data" element={<Navigate to="identities" replace />} />
                <Route path="data/identities" element={<IdentitiesPage />} />
                <Route path="data/catalog" element={<CatalogPage />} />
                <Route path="data/activity" element={<ActivityPage />} />
                <Route path="settings" element={<Navigate to="portfolio" replace />} />
                <Route path="settings/portfolio" element={<PortfolioSettingsPage />} />
                <Route path="settings/security" element={<SecuritySettingsPage />} />
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
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function Bootstrap() {
  const navigate = useNavigate();
  const portfolios = usePortfolios();
  const items = portfolios.data?.items ?? [];
  const remembered = readPreference(LAST_PORTFOLIO_KEY);
  const next = items.find((item) => item.id === remembered) ?? items[0];
  useEffect(() => {
    if (next) {
      storePreference(LAST_PORTFOLIO_KEY, next.id);
      void navigate(`/p/${next.id}/overview`, { replace: true });
    }
  }, [navigate, next]);
  if (portfolios.error || (portfolios.isSuccess && !next)) {
    return (
      <p>
        Plutus could not restore the session. <a href="/login">Sign in</a>
      </p>
    );
  }
  return <RootRedirect />;
}

function PortfolioRedirect({ to }: { to: string }) {
  const { portfolioId = "" } = useParams();
  return <Navigate to={`/p/${portfolioId}/${to}`} replace />;
}
