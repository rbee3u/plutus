import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router";
import { invalidatePortfolioReads } from "@/api";
import { useSessionScope } from "./session";

/** Hide mixed revisions and refresh the parent and children as one read family. */
export function useValuationRevision(
  expected: string | null | undefined,
  actual: string | null | undefined,
  explicitPortfolioId?: string,
): boolean {
  const { portfolioId: routePortfolioId = "" } = useParams();
  const portfolioId = explicitPortfolioId ?? routePortfolioId;
  const { userId } = useSessionScope();
  const client = useQueryClient();
  const matches = actual === undefined || (expected ?? null) === actual;
  useEffect(() => {
    if (!matches && userId && portfolioId) {
      void invalidatePortfolioReads(client, userId, portfolioId);
    }
  }, [matches, expected, actual, client, userId, portfolioId]);
  return matches;
}
