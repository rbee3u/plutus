import { useState, useId } from "react";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { getHistoryWindow } from "@/api/generated/sdk.gen";
import type { HistoryWindow, PeriodKind } from "@/api/generated/types.gen";
import { usePortfolio } from "@/shared/lib/portfolio";
import { readChoice, useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";

export const HISTORY_PRESETS = ["1d", "3d", "1w", "2w", "1Month", "1Year"] as const;
export type HistoryPreset = (typeof HISTORY_PRESETS)[number];
export const HISTORY_PAGE_SIZES = [10, 20, 50] as const;

export function presetPeriod(preset: HistoryPreset): PeriodKind {
  return preset === "1Year" ? "month" : preset === "1Month" ? "week" : "day";
}

export function historyGranularity(from: string, to: string) {
  const days = (Date.parse(to) - Date.parse(from)) / 86_400_000;
  return days <= 3 ? "raw" : days <= 45 ? "hour" : "day";
}

/** Wall-clock text only; interpretation happens on the server in the Portfolio timezone. */
export function historyLocalValue(iso: string, timezone: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? "";
  return `${value("year")}-${value("month")}-${value("day")}T${value("hour")}:${value("minute")}:${value("second")}`;
}

export function useHistoryWindow() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const portfolio = usePortfolio(portfolioId);
  const { params, update: merge } = useSearchState();
  // History has independent pagers: unrelated URL edits must not reset them.
  const update = (next: Record<string, string>) =>
    merge({ offset: params.get("offset") ?? "", ...next });
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const custom = Boolean(from || to);
  const preset = readChoice(params.get("range"), HISTORY_PRESETS, "1w");
  const timezone = portfolio.data?.item.timezone;
  const mountId = useId();
  const [anchor, setAnchor] = useState(0);
  const [applyError, setApplyError] = useState<Error | null>(null);
  const [applying, setApplying] = useState(false);
  const window = useQuery({
    queryKey: [
      "history-window",
      userId,
      portfolioId,
      timezone,
      custom ? from : preset,
      to,
      mountId,
      anchor,
    ],
    enabled: Boolean(userId && portfolioId && timezone),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
    queryFn: async (): Promise<HistoryWindow> => {
      if (custom && /(?:Z|[+-]\d\d:\d\d)$/.test(from) && /(?:Z|[+-]\d\d:\d\d)$/.test(to)) {
        if (
          !Number.isFinite(Date.parse(from)) ||
          !Number.isFinite(Date.parse(to)) ||
          Date.parse(to) <= Date.parse(from)
        ) {
          throw new Error("The end must be after a valid start.");
        }
        return { context: { portfolioId, timezone: timezone! }, from, to };
      }
      return unwrapOrRedirect(
        await getHistoryWindow({
          path: { portfolioId },
          query: custom ? { fromLocal: from, toLocal: to } : { preset },
        }),
        redirect,
      );
    },
  });
  const resets = { pointsOffset: "", closesOffset: "", changesOffset: "", point: "" };
  function selectPreset(next: HistoryPreset) {
    setApplyError(null);
    setAnchor((value) => value + 1);
    update({
      ...resets,
      range: next,
      from: "",
      to: "",
      granularity: "",
      periodKind: presetPeriod(next),
    });
  }
  async function apply(fromLocal: string, toLocal: string) {
    setApplyError(null);
    setApplying(true);
    try {
      const resolved = unwrapOrRedirect(
        await getHistoryWindow({ path: { portfolioId }, query: { fromLocal, toLocal } }),
        redirect,
      );
      update({ ...resets, range: "", from: resolved.from, to: resolved.to, granularity: "" });
    } catch (cause) {
      setApplyError(cause instanceof Error ? cause : new Error("Enter a valid range."));
    } finally {
      setApplying(false);
    }
  }
  return {
    portfolioId,
    userId,
    redirect,
    params,
    update,
    preset: custom ? null : preset,
    defaultPeriod: presetPeriod(preset),
    window: window.data,
    error: portfolio.error ?? window.error,
    applyError,
    applying,
    busy: portfolio.isPending || window.isFetching,
    selectPreset,
    apply,
    retry: () => {
      void (portfolio.error ? portfolio.refetch() : window.refetch());
    },
  };
}

export function useHistoryPager(prefix: "points" | "closes" | "changes", total?: number) {
  const { params, update: merge } = useSearchState();
  const offsetKey = `${prefix}Offset`;
  const limitKey = `${prefix}Limit`;
  const rawOffset = Number(params.get(offsetKey) ?? "0");
  const requested =
    Number.isSafeInteger(rawOffset) && rawOffset >= 0 && rawOffset <= 2_147_483_647 ? rawOffset : 0;
  const rawLimit = Number(params.get(limitKey) ?? "20");
  const limit = HISTORY_PAGE_SIZES.find((size) => size === rawLimit) ?? 20;
  const offset =
    total === undefined
      ? requested
      : Math.min(requested, Math.max(0, Math.ceil(total / limit) - 1) * limit);
  const change = (next: Record<string, string>) =>
    merge({ offset: params.get("offset") ?? "", ...next });
  return {
    offset,
    limit,
    onChange: (next: number) =>
      change({ [offsetKey]: String(next), ...(prefix === "points" ? { point: "" } : {}) }),
    onLimitChange: (next: number) =>
      change({
        [limitKey]: String(next),
        [offsetKey]: "",
        ...(prefix === "points" ? { point: "" } : {}),
      }),
  };
}
