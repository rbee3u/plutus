import type { QueryClient } from "@tanstack/react-query";

/**
 * A Portfolio-scoped query key: `[family, userId, portfolioId, ...args]`. Every
 * Portfolio-scoped family shares this prefix so one invalidation call covers
 * them all.
 */
export type PortfolioKey = readonly [string, string | undefined, string, ...unknown[]];

function scoped(family: string) {
  return (userId: string | undefined, portfolioId: string, ...args: unknown[]): PortfolioKey => [
    family,
    userId,
    portfolioId,
    ...args,
  ];
}

/** Named builders for every query family the frontend caches. */
export const keys = {
  authMe: () => ["auth", "me"] as const,
  platforms: (userId?: string) => ["platforms", userId] as const,
  portfolio: scoped("portfolio"),
  overview: scoped("overview"),
  overviewChartWindow: scoped("overview-chart-window"),
  identities: scoped("identities"),
  balances: scoped("balances"),
  positions: scoped("positions"),
  dualPositions: scoped("dual-positions"),
  earnPositions: scoped("earn-positions"),
  accountBalances: scoped("account-balances"),
  accountPositions: scoped("account-positions"),
  accountDualPositions: scoped("account-dual-positions"),
  accountEarnPositions: scoped("account-earn-positions"),
  manualBalances: scoped("manual-balances"),
  manualEditor: scoped("manual-editor"),
  manualSymbols: scoped("manual-symbols"),
  historyWindow: scoped("history-window"),
  valuationPoints: scoped("valuation-points"),
  valuationPointPage: scoped("valuation-point-page"),
  overviewChartPoints: scoped("overview-chart-points"),
  valuationPoint: scoped("valuation-point"),
  periodCloses: scoped("period-closes"),
  valueChanges: scoped("value-changes"),
  customValueChange: scoped("custom-value-change"),
  boundaryEvents: scoped("boundary-events"),
  collectionCycles: scoped("collection-cycles"),
  collectionCycle: scoped("collection-cycle"),
  jobs: scoped("jobs"),
  job: scoped("job"),
  auditEvents: scoped("audit-events"),
  catalogListings: scoped("catalog-listings"),
  catalogInstruments: scoped("catalog-instruments"),
  capitalFlows: scoped("capital-flows"),
  capitalFlow: scoped("capital-flow"),
} as const;

/** Families that read Valuation Points and change after a Collection Cycle. */
export const PORTFOLIO_READ_FAMILIES = [
  "overview",
  "overview-chart-window",
  "overview-chart-points",
  "history-window",
  "valuation-point-page",
  "balances",
  "positions",
  "dual-positions",
  "earn-positions",
  "account-balances",
  "account-positions",
  "account-dual-positions",
  "account-earn-positions",
  "manual-balances",
  "manual-symbols",
  "valuation-points",
  "valuation-point",
  "period-closes",
  "value-changes",
  "custom-value-change",
  "boundary-events",
  "collection-cycles",
  "collection-cycle",
  "jobs",
  "identities",
  "catalog-listings",
  "catalog-instruments",
  "capital-flows",
  "audit-events",
] as const;

export async function invalidatePortfolioReads(
  queryClient: QueryClient,
  userId: string | undefined,
  portfolioId: string,
): Promise<void> {
  await Promise.all(
    PORTFOLIO_READ_FAMILIES.map((family) =>
      queryClient.invalidateQueries({ queryKey: [family, userId, portfolioId] }),
    ),
  );
}
