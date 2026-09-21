import { toDatetimeLocalValue } from "@/shared/lib/datetime";
import { useEffect } from "react";
import { useSearchParams } from "react-router";

const DEFAULT_LOOKBACK_DAYS = 45;

export function defaultHistoryWindow(
  cutoffIso?: string | null,
  lookbackDays = DEFAULT_LOOKBACK_DAYS,
): { from: string; to: string } {
  let to = cutoffIso ? new Date(cutoffIso) : new Date();
  if (Number.isNaN(to.getTime())) {
    to = new Date();
  }
  const from = new Date(to.getTime() - lookbackDays * 24 * 60 * 60 * 1000);
  return {
    from: toDatetimeLocalValue(from.toISOString()),
    to: toDatetimeLocalValue(to.toISOString()),
  };
}

export function useHistorySearchWindow(cutoffIso: string | null | undefined) {
  const [params, setParams] = useSearchParams();
  const fallback = defaultHistoryWindow(cutoffIso);
  const fromParam = params.get("from");
  const toParam = params.get("to");
  const from = fromParam || fallback.from;
  const to = toParam || fallback.to;

  useEffect(() => {
    if (fromParam && toParam) {
      return;
    }
    setParams(
      (current) => {
        const merged = new URLSearchParams(current);
        if (!merged.get("from")) {
          merged.set("from", fallback.from);
        }
        if (!merged.get("to")) {
          merged.set("to", fallback.to);
        }
        return merged;
      },
      { replace: true },
    );
  }, [fromParam, toParam, fallback.from, fallback.to, setParams]);

  function update(next: Record<string, string>) {
    const merged = new URLSearchParams(params);
    for (const [key, value] of Object.entries(next)) {
      if (value) {
        merged.set(key, value);
      } else {
        merged.delete(key);
      }
    }
    setParams(merged, { replace: true });
  }

  return { from, to, params, update };
}
