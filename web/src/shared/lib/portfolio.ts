import { getPortfolio, keys, readEtag, unwrapOrRedirect } from "@/api";
import type { Portfolio } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

export interface PortfolioView {
  item: Portfolio;
  etag: string;
}

export function portfolioEtag(item: Portfolio): string {
  return `"portfolio:${String(item.version)}"`;
}

/**
 * The current Portfolio with its ETag; pages read the timezone from it. It
 * stays hand-rolled instead of using usePortfolioQuery because it must return
 * the response ETag alongside the item.
 */
export function usePortfolio(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: keys.portfolio(userId, portfolioId),
    enabled: Boolean(userId && portfolioId),
    queryFn: async (): Promise<PortfolioView> => {
      const result = await getPortfolio({ path: { portfolioId } });
      const item = unwrapOrRedirect(result, redirect);
      return { item, etag: readEtag(result.response, portfolioEtag(item)) };
    },
  });
}
