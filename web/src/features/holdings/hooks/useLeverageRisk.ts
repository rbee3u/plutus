import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { keys, listPositions, unwrapOrRedirect } from "@/api";
import type { Overview, PositionRow } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import { buildLeverageRiskMap, type LeverageRiskMap } from "../lib/leverageRisk";
import { supportsRealLeverage } from "../lib/realLeverage";

const POSITION_PAGE_LIMIT = 500;

type RiskPositions = {
  items: PositionRow[];
  cycleId: string;
};

/**
 * One cycle-scoped bulk read of stored positions for the Overview warning.
 * It never calls an exchange. Every page is required; a partial read leaves
 * the warning map empty and surfaces the neutral failure state.
 */
export function useLeverageRisk(
  portfolioId: string,
  tree: Overview,
): {
  risk: LeverageRiskMap;
  isError: boolean;
} {
  const { userId, redirect } = useSessionScope();
  const cycleId = tree.context.cycleId ?? "";
  const accounts = useMemo(
    () =>
      tree.platforms.flatMap((platform) =>
        platform.identities.flatMap((identity) => identity.accounts),
      ),
    [tree],
  );
  const hasUnifiedAccount = useMemo(
    () => accounts.some((account) => supportsRealLeverage(account)),
    [accounts],
  );

  const query = useQuery({
    queryKey: keys.leverageRisk(userId, portfolioId, cycleId),
    enabled: Boolean(userId && portfolioId && cycleId && hasUnifiedAccount),
    staleTime: Infinity,
    queryFn: async (): Promise<RiskPositions> => {
      const items: PositionRow[] = [];
      let offset = 0;
      for (;;) {
        const data = unwrapOrRedirect(
          await listPositions({
            path: { portfolioId },
            query: { cycleId, limit: POSITION_PAGE_LIMIT, offset },
          }),
          redirect,
        );
        if (data.context.cycleId !== cycleId) {
          throw new Error("The positions response does not match the selected valuation point.");
        }
        items.push(...data.items);
        if (items.length >= data.page.total) {
          break;
        }
        if (data.items.length === 0) {
          throw new Error("The positions response did not advance.");
        }
        offset += data.items.length;
      }
      return { items, cycleId };
    },
  });

  const risk = useMemo<LeverageRiskMap>(() => {
    if (!query.isSuccess || !query.data) {
      return new Map();
    }
    const positionsByAccount = new Map<string, PositionRow[]>();
    for (const item of query.data.items) {
      const current = positionsByAccount.get(item.accountId);
      if (current) current.push(item);
      else positionsByAccount.set(item.accountId, [item]);
    }
    return buildLeverageRiskMap(accounts, positionsByAccount, true);
  }, [accounts, query.data, query.isSuccess]);

  return { risk, isError: query.isError };
}
