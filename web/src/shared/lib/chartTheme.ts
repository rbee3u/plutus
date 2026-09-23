import type { EChartsCoreOption } from "echarts/core";

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
  const style = getComputedStyle(document.documentElement);
  const token = (name: string) => style.getPropertyValue(name).trim();
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

const tooltipNumber = new Intl.NumberFormat("en-US", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const axisNumber = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 1,
});

/**
 * Applies the design tokens to a chart option: quiet axes, a single-series
 * chart without a legend, line series without markers, and bars coloured by
 * sign. Page options override anything set here.
 */
export function withChartTheme(option: EChartsCoreOption): EChartsCoreOption {
  const tokens = readChartTokens();
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
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
      valueFormatter: (value: unknown) =>
        typeof value === "number" ? tooltipNumber.format(value) : "—",
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
    xAxis: applyAxis(option.xAxis, tokens, "x"),
    yAxis: applyAxis(option.yAxis, tokens, "y"),
    series,
  };
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
        color: (params: { value?: unknown }) =>
          typeof params.value === "number" && params.value < 0 ? tokens.negative : tokens.accent,
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
): EChartsCoreOption["xAxis"] {
  if (Array.isArray(axis)) {
    return axis.map((item) => styleAxis(item, tokens, which));
  }
  return styleAxis(axis, tokens, which);
}

function styleAxis(axis: unknown, tokens: ChartTokens, which: "x" | "y"): Record<string, unknown> {
  const item = asRecord(axis);
  const label = asRecord(item.axisLabel);
  return {
    ...item,
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
      ...(which === "y" ? { formatter: (value: number) => axisNumber.format(value) } : {}),
      ...label,
    },
    splitLine: {
      show: which === "y",
      lineStyle: { color: tokens.lineSoft },
      ...asRecord(item.splitLine),
    },
  };
}
