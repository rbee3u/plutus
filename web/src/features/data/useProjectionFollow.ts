import { getJob, getOverview } from "@/api/generated/sdk.gen";
import type { Job, Overview } from "@/api/generated/types.gen";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { QueryClient, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";

const TERMINAL: ReadonlySet<Job["status"]> = new Set(["succeeded", "failed", "cancelled"]);
const READ_KEYS = [
  "overview",
  "balances",
  "positions",
  "exposures",
  "valuations",
  "value-changes",
  "period-summaries",
] as const;

export function useProjectionFollow(portfolioId: string) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const queryClient = useQueryClient();
  const [job, setJob] = useState<Job | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [watchId, setWatchId] = useState<string | null>(null);
  const previousGeneration = useRef("");
  const inFlight = useRef(false);

  const follow = useCallback((next: Job, generationId?: string | null) => {
    if (generationId) {
      previousGeneration.current = generationId;
    }
    setJob(next);
    if (TERMINAL.has(next.status)) {
      setWatchId(null);
      return;
    }
    setWatchId(next.id);
    if (next.kind === "projection") {
      setNotice("A projection rebuild is running.");
    }
  }, []);

  useEffect(() => {
    if (!watchId || !portfolioId) {
      return;
    }
    let cancelled = false;
    let timer = 0;
    let delay = 1000;
    const redirect = () => {
      void navigate("/login", { replace: true });
    };

    const stopTimer = () => {
      window.clearTimeout(timer);
    };

    const schedule = () => {
      stopTimer();
      timer = window.setTimeout(() => {
        void poll();
      }, delay);
    };

    const finish = async (latest: Job) => {
      setJob(latest);
      if (latest.kind === "projection" && latest.status === "succeeded") {
        setNotice("Waiting for the new projection generation.");
        await waitForGeneration(
          queryClient,
          user.data?.id,
          portfolioId,
          previousGeneration.current,
          redirect,
        );
        if (!cancelled) {
          setNotice("Projection rebuild finished.");
        }
      } else if (latest.status === "failed") {
        setNotice(latest.lastErrorDetail ?? latest.lastErrorCode ?? "The job failed.");
      } else if (latest.status === "cancelled") {
        setNotice("The job was cancelled.");
      }
      if (!cancelled) {
        setWatchId(null);
      }
    };

    const poll = async () => {
      if (cancelled || inFlight.current) {
        return;
      }
      if (document.hidden) {
        return;
      }
      inFlight.current = true;
      try {
        const result = await getJob({ path: { portfolioId, jobId: watchId } });
        if (cancelled) {
          return;
        }
        const latest = unwrapOrRedirect(result, redirect);
        if (TERMINAL.has(latest.status)) {
          await finish(latest);
          return;
        }
        setJob(latest);
        delay = Math.min(delay * 2, 8000);
        schedule();
      } catch {
        if (!cancelled) {
          schedule();
        }
      } finally {
        inFlight.current = false;
      }
    };

    const onVisible = () => {
      if (!document.hidden) {
        void poll();
      }
    };
    document.addEventListener("visibilitychange", onVisible);
    void poll();
    return () => {
      cancelled = true;
      stopTimer();
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [navigate, portfolioId, queryClient, user.data?.id, watchId]);

  return { job, notice, follow, setNotice };
}

async function waitForGeneration(
  queryClient: QueryClient,
  userId: string | undefined,
  portfolioId: string,
  previousGenerationId: string,
  redirect: () => void,
): Promise<void> {
  const deadline = Date.now() + 30_000;
  let delay = 1000;
  while (Date.now() < deadline) {
    while (document.hidden && Date.now() < deadline) {
      await sleep(500);
    }
    const latest = await queryClient.fetchQuery({
      queryKey: ["overview", userId, portfolioId],
      queryFn: async (): Promise<Overview> =>
        unwrapOrRedirect(await getOverview({ path: { portfolioId } }), redirect),
    });
    if (!previousGenerationId || latest.context.generationId !== previousGenerationId) {
      break;
    }
    await sleep(delay);
    delay = Math.min(delay * 2, 8000);
  }
  await Promise.all(
    READ_KEYS.map((key) => queryClient.invalidateQueries({ queryKey: [key, userId, portfolioId] })),
  );
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}
