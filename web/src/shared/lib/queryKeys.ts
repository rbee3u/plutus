import type { QueryClient } from "@tanstack/react-query";

/** Query families that read from Valuation Points and change after a Collection Cycle. */
export const PORTFOLIO_READ_KEYS = [
  "overview",
  "holdings-tree",
  "balances",
  "positions",
  "account-balances",
  "account-positions",
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
] as const;

export async function invalidatePortfolioReads(
  queryClient: QueryClient,
  userId: string | undefined,
  portfolioId: string,
): Promise<void> {
  await Promise.all(
    PORTFOLIO_READ_KEYS.map((key) =>
      queryClient.invalidateQueries({ queryKey: [key, userId, portfolioId] }),
    ),
  );
}
