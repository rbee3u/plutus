import { useState, useId } from "react";
import { useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { getHistoryWindow, keys, unwrapOrRedirect } from "@/api";
import type { HistoryWindow } from "@/api";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import { useSessionScope } from "@/shared/lib/session";
import {
  DEFAULT_TIME_WINDOW_PRESET,
  normalizeTimeWindowPreset,
  presetPeriod,
  type TimeWindowPreset,
} from "./timeWindow";

export interface UseTimeWindowOptions {
  defaultPreset?: TimeWindowPreset;
  onSelectPreset?: (next: TimeWindowPreset) => Record<string, string>;
  onApply?: () => Record<string, string>;
}

export function useTimeWindow(options: UseTimeWindowOptions = {}) {
  const { defaultPreset = DEFAULT_TIME_WINDOW_PRESET, onSelectPreset, onApply } = options;
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const portfolio = usePortfolio(portfolioId);
  const { params, update: merge } = useSearchState();

  const update = (next: Record<string, string>) =>
    merge({ offset: params.get("offset") ?? "", ...next });
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const custom = Boolean(from || to);
  const preset = normalizeTimeWindowPreset(params.get("range")) ?? defaultPreset;
  const timezone = portfolio.data?.timezone;
  const mountId = useId();
  const [anchor, setAnchor] = useState(0);
  const [applyError, setApplyError] = useState<Error | null>(null);
  const [applying, setApplying] = useState(false);

  const window = useQuery({
    queryKey: keys.historyWindow(
      userId,
      portfolioId,
      timezone,
      custom ? from : preset,
      to,
      mountId,
      anchor,
    ),
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

  function selectPreset(next: TimeWindowPreset) {
    setApplyError(null);
    setAnchor((value) => value + 1);
    const extra = onSelectPreset ? onSelectPreset(next) : {};
    update({
      range: next,
      from: "",
      to: "",
      ...extra,
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
      const extra = onApply ? onApply() : {};
      update({
        range: "",
        from: resolved.from,
        to: resolved.to,
        ...extra,
      });
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
