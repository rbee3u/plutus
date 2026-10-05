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
  /** Key builder receiving the session scope so every read stays Portfolio-scoped. */
  key: (userId: string | undefined, portfolioId: string) => QueryKey;
  /** The read itself; the hook unwraps it and handles an expired session. */
  load: (portfolioId: string) => Promise<ApiResult<T>>;
  /** Extra readiness on top of the session scope and the route Portfolio. */
  ready?: boolean;
  /** Override the route Portfolio (dialogs and embedded lists pass one). */
  portfolioId?: string;
}

/**
 * A Portfolio-scoped read: session scope, cache key, readiness, and result
 * unwrapping in one place, so features only describe their own filters.
 */
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
