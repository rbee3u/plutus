import { useQuery } from "@tanstack/react-query";
import { getAccountReport, keys, unwrapOrRedirect } from "@/api";
import type { AccountValuation } from "@/api";
import { useSessionScope } from "@/shared/lib/session";

export function useAccountReport(portfolioId: string, account: AccountValuation, cycleId: string) {
  const { userId, redirect } = useSessionScope();
  const report = useQuery({
    queryKey: keys.accountReport(
      userId,
      portfolioId,
      account.accountId,
      cycleId,
      true,
      account.reportId,
    ),
    enabled: Boolean(userId && portfolioId && account.accountId),
    queryFn: async () => {
      const data = unwrapOrRedirect(
        await getAccountReport({
          path: { portfolioId, accountId: account.accountId },
          query: { ...(cycleId ? { cycleId } : {}), includePendingManual: true },
        }),
        redirect,
      );
      if (
        (data.context.cycleId ?? "") !== cycleId ||
        (data.account.reportId ?? null) !== (account.reportId ?? null)
      ) {
        throw new Error(
          "The account response does not match the selected valuation point and report.",
        );
      }
      return data;
    },
  });
  return {
    report,
    isManual: account.accountType === "manual",
    isDual: account.accountType === "dual_investment",
    isEarn: account.accountType === "earn",
  };
}
