import { getPortfolio } from "@/api/generated/sdk.gen";
import type { Portfolio } from "@/api/generated/types.gen";
import { readEtag, unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

export interface PortfolioView {
  item: Portfolio;
  etag: string;
}

export function portfolioEtag(item: Portfolio): string {
  return `"portfolio:${String(item.version)}"`;
}

/** The current Portfolio with its ETag; pages read the timezone from it. */
export function usePortfolio(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: ["portfolio", userId, portfolioId],
    enabled: Boolean(userId && portfolioId),
    queryFn: async (): Promise<PortfolioView> => {
      const result = await getPortfolio({ path: { portfolioId } });
      const item = unwrapOrRedirect(result, redirect);
      return { item, etag: readEtag(result.response, portfolioEtag(item)) };
    },
  });
}
