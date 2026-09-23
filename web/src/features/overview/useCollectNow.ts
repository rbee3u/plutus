import { createCollectionCycle, getCollectionCycle } from "@/api/generated/sdk.gen";
import type { CollectionCycle, JobStatus } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { newIdempotencyKey } from "@/shared/lib/id";
import { invalidatePortfolioReads } from "@/shared/lib/queryKeys";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

const TERMINAL: ReadonlySet<JobStatus> = new Set(["succeeded", "failed", "cancelled"]);
const FOLLOW_INTERVAL_MS = 2000;

export function isTerminalStatus(status: JobStatus | undefined): boolean {
  return status !== undefined && TERMINAL.has(status);
}

/**
 * Enqueues a manual Collection Cycle and follows it until it finishes, then
 * invalidates every read that depends on the new Valuation Point.
 */
export function useCollectNow(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const idempotencyKey = useRef(newIdempotencyKey());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [started, setStarted] = useState<CollectionCycle | null>(null);
  const watchId =
    started && started.portfolioId === portfolioId && !isTerminalStatus(started.status)
      ? started.id
      : null;

  const followed = useQuery({
    queryKey: ["collection-cycle", userId, portfolioId, watchId],
    enabled: Boolean(userId && portfolioId && watchId),
    refetchInterval: (query) =>
      isTerminalStatus(query.state.data?.cycle.status) ? false : FOLLOW_INTERVAL_MS,
    queryFn: async () =>
      unwrapOrRedirect(
        await getCollectionCycle({ path: { portfolioId, cycleId: watchId ?? "" } }),
        redirect,
      ),
  });

  const latest = followed.data?.cycle;
  const own = started?.portfolioId === portfolioId ? started : null;
  const cycle = latest && latest.id === own?.id ? latest : own;
  const finishedId = cycle && isTerminalStatus(cycle.status) ? cycle.id : null;

  useEffect(() => {
    if (!finishedId) {
      return;
    }
    void invalidatePortfolioReads(queryClient, userId, portfolioId);
  }, [finishedId, portfolioId, queryClient, userId]);

  async function start() {
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await createCollectionCycle({
        path: { portfolioId },
        headers: { "Idempotency-Key": idempotencyKey.current },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The Collection Cycle could not be queued."));
        return;
      }
      idempotencyKey.current = newIdempotencyKey();
      setStarted(result.data);
      await queryClient.invalidateQueries({ queryKey: ["collection-cycles", userId, portfolioId] });
    } catch (cause) {
      setError(problemMessage(cause, "The Collection Cycle could not be queued."));
    } finally {
      setBusy(false);
    }
  }

  return { start, busy, error, cycle };
}
