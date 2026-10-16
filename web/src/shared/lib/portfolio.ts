import { getPortfolio, keys } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function usePortfolio(portfolioId: string) {
  return usePortfolioQuery({
    portfolioId,
    key: keys.portfolio,
    load: (portfolioId) => getPortfolio({ path: { portfolioId } }),
  });
}
