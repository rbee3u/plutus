import {
  getAuthSessions,
  keys,
  problemMessage,
  SessionExpiredError,
  unwrapOrRedirect,
} from "@/api";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { useSessionScope } from "@/shared/lib/session";

export function sessionsOptions(
  userId: string | undefined,
  redirect: () => void,
  onError: (error: string) => void,
) {
  return queryOptions({
    queryKey: keys.authSessions(userId),
    enabled: Boolean(userId),
    // Match the previous page lifetime: load on entry and after revocation only.
    gcTime: 0,
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    queryFn: async ({ signal }) => {
      try {
        const result = await getAuthSessions({ signal });
        if (result.response?.status !== 401 && (result.error || !result.data)) {
          throw new Error(problemMessage(result.error, "Sessions could not be loaded."));
        }
        return unwrapOrRedirect(result, redirect);
      } catch (cause) {
        if (!(cause instanceof SessionExpiredError)) {
          onError(
            problemMessage(cause, "Sessions could not be loaded. Check the network and retry."),
          );
        }
        throw cause;
      }
    },
  });
}

export function useSessions(onError: (error: string) => void) {
  const { userId, redirect } = useSessionScope();
  return useQuery(sessionsOptions(userId, redirect, onError));
}
