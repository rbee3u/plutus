import { unwrapOrRedirect, type ApiResult } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import {
  useQuery,
  type QueryKey,
  type UseQueryOptions,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useParams } from "react-router";

interface PortfolioQueryOptions<T> extends Omit<
  UseQueryOptions<T>,
  "queryKey" | "queryFn" | "enabled"
> {
  key: (userId: string | undefined, portfolioId: string) => QueryKey;
  load: (portfolioId: string) => Promise<ApiResult<T>>;
  /** Extra readiness on top of the session scope and the route portfolio. */
  ready?: boolean;
  /** Override the route portfolio (dialogs and embedded lists pass one). */
  portfolioId?: string;
}

/** Scope reads to the session and portfolio, handling session expiry. */
export function usePortfolioQuery<T>({
  key,
  load,
  ready = true,
  portfolioId: explicitPortfolioId,
  ...options
}: PortfolioQueryOptions<T>): UseQueryResult<T> {
  const { portfolioId: routePortfolioId = "" } = useParams();
  const portfolioId = explicitPortfolioId ?? routePortfolioId;
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: key(userId, portfolioId),
    enabled: Boolean(userId && portfolioId && ready),
    queryFn: async (): Promise<T> => unwrapOrRedirect(await load(portfolioId), redirect),
    ...options,
  });
}
