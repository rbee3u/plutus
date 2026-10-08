import { useQuery } from "@tanstack/react-query";
import {
  keys,
  listAccountBalances,
  listAccountDualPositions,
  listAccountEarnPositions,
  listAccountPositions,
  unwrapOrRedirect,
} from "@/api";
import type { AccountValuation, ReadContext } from "@/api";
import { useSessionScope } from "@/shared/lib/session";

/** Own the independent reads for one visible Account Report. */
export function useAccountReport(portfolioId: string, account: AccountValuation, pointId: string) {
  const { userId, redirect } = useSessionScope();
  const accountId = account.accountId;
  const balances = useQuery({
    queryKey: keys.accountBalances(userId, portfolioId, accountId, pointId, account.reportId),
    enabled: Boolean(userId && portfolioId && accountId),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(await listAccountBalances({ path: { portfolioId, accountId } }), redirect),
        account,
        pointId,
      ),
  });
  const isManual = account.accountType === "manual";
  const isDual = account.accountType === "dual_investment";
  const isEarn = account.accountType === "earn";
  const positions = useQuery({
    queryKey: keys.accountPositions(userId, portfolioId, accountId, pointId, account.reportId),
    enabled: Boolean(userId && portfolioId && accountId && !isDual && !isEarn && !isManual),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountPositions({ path: { portfolioId, accountId }, query: { pointId } }),
          redirect,
        ),
        account,
        pointId,
      ),
  });
  const dualPositions = useQuery({
    queryKey: keys.accountDualPositions(userId, portfolioId, accountId, pointId, account.reportId),
    enabled: Boolean(userId && portfolioId && isDual),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountDualPositions({ path: { portfolioId, accountId } }),
          redirect,
        ),
        account,
        pointId,
      ),
  });
  const earnPositions = useQuery({
    queryKey: keys.accountEarnPositions(userId, portfolioId, accountId, pointId, account.reportId),
    enabled: Boolean(userId && portfolioId && isEarn),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountEarnPositions({ path: { portfolioId, accountId } }),
          redirect,
        ),
        account,
        pointId,
      ),
  });
  return { balances, positions, dualPositions, earnPositions, isManual, isDual, isEarn };
}

/** A newer latest-only response must not describe an older visible Account row. */
function checkAccountReport<T extends { context: ReadContext; account: AccountValuation }>(
  data: T,
  account: AccountValuation,
  pointId: string,
): T {
  if (
    data.context.pointId !== pointId ||
    (data.account.reportId ?? null) !== (account.reportId ?? null)
  ) {
    throw new Error(
      "This Account changed since the displayed Valuation Point. Reload the page to read its latest Account Report.",
    );
  }
  return data;
}
