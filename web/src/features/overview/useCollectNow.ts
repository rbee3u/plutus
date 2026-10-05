import {
  createCollectionCycle,
  getCollectionCycle,
  invalidatePortfolioReads,
  keys,
  problemMessage,
  refreshCsrf,
} from "@/api";
import type { CollectionCycle, CollectionCycleDetail, JobStatus } from "@/api";
import { newIdempotencyKey } from "@/shared/lib/id";
import { useSessionScope } from "@/shared/lib/session";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

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

  const followed = usePortfolioQuery<CollectionCycleDetail>({
    key: (userId, portfolioId) => keys.collectionCycle(userId, portfolioId, watchId),
    ready: Boolean(watchId),
    load: (portfolioId) => getCollectionCycle({ path: { portfolioId, cycleId: watchId ?? "" } }),
    refetchInterval: (query) =>
      isTerminalStatus(query.state.data?.cycle.status) ? false : FOLLOW_INTERVAL_MS,
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
      await queryClient.invalidateQueries({ queryKey: keys.collectionCycles(userId, portfolioId) });
    } catch (cause) {
      setError(problemMessage(cause, "The Collection Cycle could not be queued."));
    } finally {
      setBusy(false);
    }
  }

  return { start, busy, error, cycle };
}
