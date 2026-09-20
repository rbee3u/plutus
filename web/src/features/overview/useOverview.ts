import { getOverview } from "@/api/generated/sdk.gen";
import type { Overview } from "@/api/generated/types.gen";
import { SessionExpiredError, unwrapData, useCurrentUser } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router";

export function useOverview(portfolioId: string) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  return useQuery({
    queryKey: ["overview", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<Overview> => {
      const result = await getOverview({ path: { portfolioId } });
      try {
        return unwrapData(result);
      } catch (error) {
        if (error instanceof SessionExpiredError) {
          void navigate("/login", { replace: true });
        }
        throw error;
      }
    },
  });
}
