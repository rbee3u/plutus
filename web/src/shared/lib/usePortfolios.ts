import { keys, listPortfolios, unwrapOrRedirect } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import { queryOptions, useQuery } from "@tanstack/react-query";

export function portfoliosOptions(userId: string | undefined, redirect: () => void) {
  return queryOptions({
    queryKey: keys.portfolios(userId),
    enabled: Boolean(userId),
    queryFn: async () => unwrapOrRedirect(await listPortfolios(), redirect),
  });
}

/** User-scoped list shared by bootstrap, the switcher, and Portfolio mutations. */
export function usePortfolios() {
  const { userId, redirect } = useSessionScope();
  return useQuery(portfoliosOptions(userId, redirect));
}
