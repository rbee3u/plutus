import { useRef } from "react";
import { useValuationRevision } from "@/shared/lib/useValuationRevision";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  keys,
  invalidatePortfolioReads,
  listAccountBalances,
  listAccountDualPositions,
  listAccountEarnPositions,
  listAccountPositions,
  unwrapOrRedirect,
} from "@/api";
import type { AccountValuation, ReadContext } from "@/api";
import { useSessionScope } from "@/shared/lib/session";

/** Own the independent reads for one visible Account Report. */
export function useAccountReport(
  portfolioId: string,
  account: AccountValuation,
  cycleId: string,
  valuationRevision: string | null,
) {
  const { userId, redirect } = useSessionScope();
  const client = useQueryClient();
  const refreshed = useRef<string | null>(null);
  const refresh = () => {
    const visible = `${cycleId}/${valuationRevision}/${account.reportId}`;
    if (userId && refreshed.current !== visible) {
      refreshed.current = visible;
      void invalidatePortfolioReads(client, userId, portfolioId);
    }
  };
  const accountId = account.accountId;
  const balances = useQuery({
    queryKey: keys.accountBalances(
      userId,
      portfolioId,
      accountId,
      cycleId,
      account.reportId,
      valuationRevision,
    ),
    enabled: Boolean(userId && portfolioId && accountId),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(await listAccountBalances({ path: { portfolioId, accountId } }), redirect),
        account,
        cycleId,
        refresh,
      ),
  });
  const isManual = account.accountType === "manual";
  const isDual = account.accountType === "dual_investment";
  const isEarn = account.accountType === "earn";
  const positions = useQuery({
    queryKey: keys.accountPositions(
      userId,
      portfolioId,
      accountId,
      cycleId,
      account.reportId,
      valuationRevision,
    ),
    enabled: Boolean(userId && portfolioId && accountId && !isDual && !isEarn && !isManual),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountPositions({ path: { portfolioId, accountId }, query: { cycleId } }),
          redirect,
        ),
        account,
        cycleId,
        refresh,
      ),
  });
  const dualPositions = useQuery({
    queryKey: keys.accountDualPositions(
      userId,
      portfolioId,
      accountId,
      cycleId,
      account.reportId,
      valuationRevision,
    ),
    enabled: Boolean(userId && portfolioId && isDual),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountDualPositions({ path: { portfolioId, accountId } }),
          redirect,
        ),
        account,
        cycleId,
        refresh,
      ),
  });
  const earnPositions = useQuery({
    queryKey: keys.accountEarnPositions(
      userId,
      portfolioId,
      accountId,
      cycleId,
      account.reportId,
      valuationRevision,
    ),
    enabled: Boolean(userId && portfolioId && isEarn),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountEarnPositions({ path: { portfolioId, accountId } }),
          redirect,
        ),
        account,
        cycleId,
        refresh,
      ),
  });
  const balancesMatch = useValuationRevision(
    valuationRevision,
    balances.data?.context.valuationRevision,
    portfolioId,
  );
  const positionsMatch = useValuationRevision(
    valuationRevision,
    positions.data?.context.valuationRevision,
    portfolioId,
  );
  const dualMatch = useValuationRevision(
    valuationRevision,
    dualPositions.data?.context.valuationRevision,
    portfolioId,
  );
  const earnMatch = useValuationRevision(
    valuationRevision,
    earnPositions.data?.context.valuationRevision,
    portfolioId,
  );
  return {
    balances: { ...balances, data: balancesMatch ? balances.data : undefined },
    positions: { ...positions, data: positionsMatch ? positions.data : undefined },
    dualPositions: { ...dualPositions, data: dualMatch ? dualPositions.data : undefined },
    earnPositions: { ...earnPositions, data: earnMatch ? earnPositions.data : undefined },
    isManual,
    isDual,
    isEarn,
  };
}

/** A newer latest-only response must not describe an older visible Account row. */
function checkAccountReport<T extends { context: ReadContext; account: AccountValuation }>(
  data: T,
  account: AccountValuation,
  cycleId: string,
  refresh: () => void,
): T {
  if (
    (data.context.cycleId ?? "") !== cycleId ||
    (data.account.reportId ?? null) !== (account.reportId ?? null)
  ) {
    refresh();
    throw new Error(
      "This Account changed since the displayed Valuation Point. Refreshing the Portfolio to read a consistent Account Report.",
    );
  }
  return data;
}
