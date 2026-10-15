import { getAuthMe, keys, resetCsrf, unwrapData, type User } from "@/api";
import { queryOptions, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { useNavigate, type NavigateFunction } from "react-router";

const generations = new WeakMap<QueryClient, number>();

/** Request callbacks retain the session lifetime in which they were created. */
export function sessionGeneration(queryClient: QueryClient): number {
  return generations.get(queryClient) ?? 0;
}

/** One authenticated user source for routes, features, and the application shell. */
export function currentUserOptions() {
  return queryOptions({
    queryKey: keys.authMe(),
    staleTime: Infinity,
    queryFn: async (): Promise<User | null> => {
      const result = await getAuthMe();
      return result.response?.status === 401 ? null : unwrapData(result).user;
    },
    retry: 1,
  });
}

export function useCurrentUser() {
  return useQuery(currentUserOptions());
}

/** Remove all cached user data before ending or replacing a browser session. */
export async function clearSession(queryClient: QueryClient): Promise<boolean> {
  const generation = sessionGeneration(queryClient) + 1;
  generations.set(queryClient, generation);
  await queryClient.cancelQueries();
  if (sessionGeneration(queryClient) !== generation) return false;
  queryClient.clear();
  resetCsrf();
  queryClient.setQueryData(keys.authMe(), null);
  return true;
}

export async function endSession(
  queryClient: QueryClient,
  navigate: NavigateFunction,
  state?: { notice: string },
): Promise<void> {
  if (!(await clearSession(queryClient))) return;
  void navigate("/login", { replace: true, state });
}

/** A late 401 from a replaced session cannot end a newly authenticated session. */
export async function expireSession(
  queryClient: QueryClient,
  generation: number,
  navigate: NavigateFunction,
): Promise<void> {
  if (sessionGeneration(queryClient) === generation) await endSession(queryClient, navigate);
}

export function useSessionActions() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const generation = sessionGeneration(queryClient);
  const end = useCallback(
    (state?: { notice: string }) => endSession(queryClient, navigate, state),
    [queryClient, navigate],
  );
  const expire = useCallback(
    () => expireSession(queryClient, generation, navigate),
    [queryClient, generation, navigate],
  );
  return { endSession: end, expireSession: expire };
}

/**
 * The current user id for query keys plus the login redirect every
 * portfolio-scoped query and mutation shares.
 */
export function useSessionScope() {
  const user = useCurrentUser();
  const { expireSession } = useSessionActions();
  const redirect = useCallback(() => {
    void expireSession();
  }, [expireSession]);
  return { userId: user.data?.id, redirect };
}
