import type { EChartsCoreOption } from "echarts/core";
import Decimal from "decimal.js";
import { formatAmount, parseDecimal } from "./amount";

export interface ChartTokens {
  ink: string;
  muted: string;
  faint: string;
  line: string;
  lineSoft: string;
  surface: string;
  accent: string;
  positive: string;
  negative: string;
  warning: string;
  font: string;
  mono: string;
}

export function readChartTokens(): ChartTokens {
  const style =
    typeof window !== "undefined" &&
    typeof document !== "undefined" &&
    typeof getComputedStyle !== "undefined"
      ? getComputedStyle(document.documentElement)
      : null;
  const token = (name: string) => style?.getPropertyValue(name).trim() || "";
  return {
    ink: token("--ink") || "#161b22",
    muted: token("--muted") || "#5d6877",
    faint: token("--faint") || "#7a8595",
    line: token("--line") || "#e2e7ee",
    lineSoft: token("--line-soft") || "#edf0f4",
    surface: token("--surface-raised") || "#ffffff",
    accent: token("--accent") || "#4f46e5",
    positive: token("--success") || "#067647",
    negative: token("--danger") || "#b42318",
    warning: token("--warning") || "#b54708",
    font: token("--font-sans") || "sans-serif",
    mono: token("--font-mono") || "monospace",
  };
}

const compactZero = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 0,
});

const compactOne = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 1,
});

const compactTwo = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 2,
});

const standardInteger = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

/** Axis ticks favor space; source-value tooltips use the financial display policy. */
export function formatAxisNumber(value: number, step?: number): string {
  const amount = parseDecimal(value);
  if (amount === null) return "—";
  if (amount.isZero()) return "0";
  const absVal = amount.abs().toNumber();
  if (amount.abs().gte(1)) {
    if (step !== undefined && step > 0) {
      let unit = 1;
      if (absVal >= 1e9) unit = 1e9;
      else if (absVal >= 1e6) unit = 1e6;
      else if (absVal >= 1e3) unit = 1e3;

      if (unit >= 1e3) {
        const stepInUnit = step / unit;
        if (stepInUnit >= 0.8) return compactZero.format(value);
        if (stepInUnit >= 0.08) return compactOne.format(value);
        if (stepInUnit >= 0.008) return compactTwo.format(value);
        return standardInteger.format(value);
      }
    }
    return compactOne.format(value);
  }
  const rounded = amount.toSignificantDigits(4, Decimal.ROUND_HALF_UP);
  if (rounded.abs().gte(1)) return compactOne.format(rounded.toNumber());
  return rounded.abs().lt("0.0001") ? rounded.toExponential() : rounded.toFixed();
}

/**
 * Applies the design tokens to a chart option: quiet axes, a single-series
 * chart without a legend, line series without markers, and bars coloured by
 * sign. Page options override anything set here.
 */
export function withChartTheme(option: EChartsCoreOption): EChartsCoreOption {
  const tokens = readChartTokens();
  const reduceMotion =
    typeof window !== "undefined" && typeof window.matchMedia !== "undefined"
      ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
      : false;
  const series = seriesList(option.series).map((item) => styleSeries(item, tokens));
  const showLegend = series.length > 1;
  return {
    ...option,
    animation: !reduceMotion,
    animationDuration: 300,
    color: option.color ?? [tokens.accent, tokens.positive, tokens.warning, tokens.negative],
    backgroundColor: "transparent",
    textStyle: { color: tokens.ink, fontFamily: tokens.font, ...asRecord(option.textStyle) },
    grid: {
      left: 8,
      right: 12,
      top: showLegend ? 36 : 16,
      bottom: 8,
      containLabel: true,
      ...asRecord(option.grid),
    },
    tooltip: {
      trigger: "axis",
      backgroundColor: tokens.surface,
      borderColor: tokens.line,
      padding: [6, 10],
      textStyle: { color: tokens.ink, fontFamily: tokens.font, fontSize: 12 },
      axisPointer: { type: "line", lineStyle: { color: tokens.line } },
      valueFormatter: (value: unknown) => (typeof value === "number" ? formatAmount(value) : "—"),
      ...asRecord(option.tooltip),
    },
    legend: {
      show: showLegend,
      top: 0,
      right: 0,
      icon: "roundRect",
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: tokens.muted, fontFamily: tokens.font, fontSize: 12 },
      ...asRecord(option.legend),
    },
    xAxis: applyAxis(option.xAxis, tokens, "x", option.series),
    yAxis: applyAxis(option.yAxis, tokens, "y", option.series),
    series,
  };
}

function extractSeriesValues(series: unknown): number[] {
  const list = seriesList(series);
  const values: number[] = [];
  for (const s of list) {
    const data = s.data;
    if (!Array.isArray(data)) continue;
    for (const item of data) {
      if (typeof item === "number" && Number.isFinite(item)) {
        values.push(item);
      } else if (item && typeof item === "object") {
        if ("value" in item) {
          const val = (item as { value?: unknown }).value;
          if (typeof val === "number" && Number.isFinite(val)) {
            values.push(val);
          } else if (Array.isArray(val) && typeof val[1] === "number" && Number.isFinite(val[1])) {
            values.push(val[1]);
          }
        } else if (Array.isArray(item) && typeof item[1] === "number" && Number.isFinite(item[1])) {
          values.push(item[1]);
        }
      }
    }
  }
  return values;
}

export function estimateSeriesStep(series: unknown, splitNumber = 5): number | undefined {
  const values = extractSeriesValues(series);
  if (values.length < 2) return undefined;
  let min = values[0];
  let max = values[0];
  for (let i = 1; i < values.length; i++) {
    const v = values[i];
    if (v < min) min = v;
    if (v > max) max = v;
  }
  const span = max - min;
  if (span <= 0) return undefined;
  return span / Math.max(1, splitNumber);
}

function seriesList(value: unknown): Record<string, unknown>[] {
  if (Array.isArray(value)) {
    return value.map(asRecord);
  }
  return value ? [asRecord(value)] : [];
}

function styleSeries(item: Record<string, unknown>, tokens: ChartTokens): Record<string, unknown> {
  if (item.type === "line") {
    return {
      showSymbol: false,
      symbol: "circle",
      symbolSize: 6,
      lineStyle: { width: 2 },
      areaStyle: { opacity: 0.08 },
      emphasis: { focus: "none" },
      ...item,
    };
  }
  if (item.type === "bar") {
    return {
      barMaxWidth: 28,
      itemStyle: {
        borderRadius: [3, 3, 0, 0],
        color: (params: { value?: unknown }) => {
          let num: number | null = null;
          if (typeof params.value === "number") {
            num = params.value;
          } else if (
            params.value &&
            typeof params.value === "object" &&
            "value" in params.value &&
            typeof params.value.value === "number"
          ) {
            num = params.value.value;
          }
          if (num === null) return tokens.accent;
          if (num > 0) return tokens.positive;
          if (num < 0) return tokens.negative;
          return tokens.muted;
        },
      },
      ...item,
    };
  }
  return item;
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function applyAxis(
  axis: EChartsCoreOption["xAxis"],
  tokens: ChartTokens,
  which: "x" | "y",
  series: unknown,
): EChartsCoreOption["xAxis"] {
  if (Array.isArray(axis)) {
    return axis.map((item) => styleAxis(item, tokens, which, series));
  }
  return styleAxis(axis, tokens, which, series);
}

function styleAxis(
  axis: unknown,
  tokens: ChartTokens,
  which: "x" | "y",
  series: unknown,
): Record<string, unknown> {
  const item = asRecord(axis);
  const label = asRecord(item.axisLabel);
  const isScaledValue = which === "y" && item.scale === true;
  const splitCount = typeof item.splitNumber === "number" ? item.splitNumber : 5;
  const step = which === "y" ? estimateSeriesStep(series, splitCount) : undefined;
  const values = which === "y" ? extractSeriesValues(series) : [];
  const isZeroVariance = values.length > 0 && values.every((v) => v === values[0]);

  return {
    ...item,
    ...(isScaledValue && {
      boundaryGap: item.boundaryGap ?? ["5%", "5%"],
      ...(isZeroVariance &&
        item.min === undefined &&
        item.max === undefined && {
          min: (val: { min: number; max: number }) =>
            val.min === 0 ? -1 : val.min > 0 ? val.min * 0.99 : val.min * 1.01,
          max: (val: { min: number; max: number }) =>
            val.max === 0 ? 1 : val.max > 0 ? val.max * 1.01 : val.max * 0.99,
        }),
    }),
    axisLine: {
      show: which === "x",
      lineStyle: { color: tokens.line },
      ...asRecord(item.axisLine),
    },
    axisTick: { show: false, ...asRecord(item.axisTick) },
    axisLabel: {
      color: tokens.muted,
      fontFamily: tokens.mono,
      fontSize: 11,
      hideOverlap: true,
      ...(which === "y" ? { formatter: (value: number) => formatAxisNumber(value, step) } : {}),
      ...label,
    },
    splitLine: {
      show: which === "y",
      lineStyle: { color: tokens.lineSoft },
      ...asRecord(item.splitLine),
    },
  };
}
