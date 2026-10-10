import { getPortfolio, keys } from "@/api";
import type { Portfolio } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function portfolioEtag(item: Portfolio): string {
  return `"portfolio:${String(item.version)}"`;
}

export function usePortfolio(portfolioId: string) {
  return usePortfolioQuery({
    portfolioId,
    key: keys.portfolio,
    load: (portfolioId) => getPortfolio({ path: { portfolioId } }),
  });
}
