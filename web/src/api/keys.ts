import type { QueryClient } from "@tanstack/react-query";

/** Keep each family identity, typed arguments, and collection policy together. */
function family<Args extends readonly unknown[]>(name: string, refreshOnCollection: boolean) {
  return {
    key: (userId: string | undefined, portfolioId: string, ...args: Args) =>
      [name, userId, portfolioId, ...args] as const,
    prefix: (userId: string | undefined, portfolioId: string) =>
      [name, userId, portfolioId] as const,
    refreshOnCollection,
  };
}

const registry = {
  portfolio: family<[]>("portfolio", false),
  overview: family<[]>("overview", true),
  overviewChartWindow: family<[preset?: string]>("overview-chart-window", true),
  overviewChartPoints: family<[from?: string, to?: string, granularity?: string]>(
    "overview-chart-points",
    true,
  ),
  identities: family<[]>("identities", true),
  holdingsEpoch: family<[]>("holdings-epoch", false),
  balances: family<
    [platform?: string, identityId?: string, symbol?: string, offset?: number, readId?: string]
  >("balances", true),
  positions: family<
    [platform?: string, identityId?: string, symbol?: string, offset?: number, readId?: string]
  >("positions", true),
  dualPositions: family<
    [platform?: string, identityId?: string, symbol?: string, offset?: number, readId?: string]
  >("dual-positions", true),
  earnPositions: family<
    [platform?: string, identityId?: string, symbol?: string, offset?: number, readId?: string]
  >("earn-positions", true),
  accountReport: family<
    [
      accountId?: string,
      cycleId?: string | null,
      includePendingManual?: boolean,
      reportId?: string | null,
    ]
  >("account-report", true),
  manualEditor: family<[identityId?: string, editorId?: string]>("manual-editor", false),
  historyWindow: family<
    [timezone?: string, fromOrPreset?: string, to?: string, mountId?: string, anchor?: number]
  >("history-window", true),
  valuationPoints: family<[from?: string, to?: string, granularity?: string]>(
    "valuation-points",
    true,
  ),
  valuationPointPage: family<[from?: string, to?: string, offset?: number, limit?: number]>(
    "valuation-point-page",
    true,
  ),
  valuationPoint: family<[cycleId?: string]>("valuation-point", true),
  periodCloses: family<[kind?: string, from?: string, to?: string]>("period-closes", true),
  valueChanges: family<[kind?: string, from?: string, to?: string]>("value-changes", true),
  customValueChange: family<[from?: string, to?: string]>("custom-value-change", true),
  boundaryEvents: family<[from?: string, to?: string, offset?: number]>("boundary-events", true),
  collectionCycles: family<[cursor?: string]>("collection-cycles", true),
  collectionCycle: family<[cycleId?: string | null]>("collection-cycle", true),
  auditEvents: family<[action?: string, resourceType?: string, cursor?: string]>(
    "audit-events",
    true,
  ),
  catalogAssets: family<[platform?: string, search?: string, status?: string, offset?: number]>(
    "catalog-assets",
    true,
  ),
  catalogFuturesContracts: family<[platform?: string, search?: string, offset?: number]>(
    "catalog-futures-contracts",
    true,
  ),
  capitalFlows: family<[from?: string, to?: string, offset?: number]>("capital-flows", true),
  capitalFlow: family<[flowId?: string]>("capital-flow", false),
} as const;

const portfolioKeys = Object.fromEntries(
  Object.entries(registry).map(([name, entry]) => [name, entry.key]),
) as { [K in keyof typeof registry]: (typeof registry)[K]["key"] };

export const keys = {
  authMe: () => ["auth", "me"] as const,
  authSessions: (userId?: string) => ["auth", "sessions", userId] as const,
  platforms: (userId?: string) => ["platforms", userId] as const,
  portfolios: (userId?: string) => ["portfolios", userId] as const,
  ...portfolioKeys,
};

/** Preserve the existing collection-dependent set; editable resources are separate. */
export async function invalidatePortfolioReads(
  queryClient: QueryClient,
  userId: string | undefined,
  portfolioId: string,
  options: { preserveHistoryWindow?: boolean } = {},
): Promise<void> {
  queryClient.setQueryData<number>(
    keys.holdingsEpoch(userId, portfolioId),
    (epoch) => (epoch ?? 0) + 1,
  );
  await Promise.all(
    Object.entries(registry)
      .filter(
        ([name, entry]) =>
          entry.refreshOnCollection && !(options.preserveHistoryWindow && name === "historyWindow"),
      )
      .map(([, entry]) =>
        queryClient.invalidateQueries({ queryKey: entry.prefix(userId, portfolioId) }),
      ),
  );
}
