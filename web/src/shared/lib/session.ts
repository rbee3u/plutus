import { getAuthMe, keys, SessionExpiredError, unwrapData, type User } from "@/api";
import { useQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import { useNavigate } from "react-router";

export function useCurrentUser() {
  const navigate = useNavigate();
  return useQuery({
    queryKey: keys.authMe(),
    queryFn: async (): Promise<User> => {
      const result = await getAuthMe();
      try {
        return unwrapData(result).user;
      } catch (error) {
        if (error instanceof SessionExpiredError) {
          void navigate("/login", { replace: true });
        }
        throw error;
      }
    },
    retry: (count, error) => !(error instanceof SessionExpiredError) && count < 1,
  });
}

/**
 * The current user id for query keys plus the login redirect every
 * portfolio-scoped query and mutation shares.
 */
export function useSessionScope() {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const redirect = useCallback(() => {
    void navigate("/login", { replace: true });
  }, [navigate]);
  return { userId: user.data?.id, redirect };
}
