import { useSearchState } from "@/shared/lib/searchParams";
import { useTimeWindow } from "@/shared/lib/useTimeWindow";
import { HISTORY_PAGE_SIZES, presetPeriod } from "@/shared/lib/timeWindow";

export function useHistoryWindow() {
  const resets = { pointsOffset: "", closesOffset: "", changesOffset: "", point: "" };
  return useTimeWindow({
    defaultPreset: "1W",
    onSelectPreset: (next) => ({
      ...resets,
      granularity: "",
      periodKind: presetPeriod(next),
    }),
    onApply: () => ({ ...resets, granularity: "" }),
  });
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
