import type { EChartsCoreOption } from "echarts/core";

export interface ChartTokens {
  ink: string;
  muted: string;
  line: string;
  surface: string;
  accent: string;
  positive: string;
  negative: string;
  warning: string;
  font: string;
}

export function readChartTokens(): ChartTokens {
  const style = getComputedStyle(document.documentElement);
  const token = (name: string) => style.getPropertyValue(name).trim();
  return {
    ink: token("--ink") || "#18202c",
    muted: token("--muted") || "#59667a",
    line: token("--line") || "#dde4ed",
    surface: token("--surface") || "#ffffff",
    accent: token("--accent") || "#4f46e5",
    positive: token("--value-positive") || "#047857",
    negative: token("--value-negative") || "#b91c1c",
    warning: token("--warning") || "#b45309",
    font: token("--font-sans") || "sans-serif",
  };
}

export function withChartTheme(option: EChartsCoreOption): EChartsCoreOption {
  const tokens = readChartTokens();
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  return {
    ...option,
    animation: !reduceMotion,
    color: option.color ?? [tokens.accent, tokens.positive, tokens.negative, tokens.warning],
    backgroundColor: "transparent",
    textStyle: { color: tokens.ink, fontFamily: tokens.font, ...asRecord(option.textStyle) },
    grid: {
      left: 16,
      right: 16,
      top: 36,
      bottom: 28,
      containLabel: true,
      ...asRecord(option.grid),
    },
    tooltip: {
      trigger: "axis",
      backgroundColor: tokens.surface,
      borderColor: tokens.line,
      textStyle: { color: tokens.ink, fontFamily: tokens.font },
      ...asRecord(option.tooltip),
    },
    legend: {
      textStyle: { color: tokens.muted, fontFamily: tokens.font },
      ...asRecord(option.legend),
    },
    xAxis: applyAxis(option.xAxis, tokens),
    yAxis: applyAxis(option.yAxis, tokens),
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function applyAxis(
  axis: EChartsCoreOption["xAxis"],
  tokens: ChartTokens,
): EChartsCoreOption["xAxis"] {
  if (Array.isArray(axis)) {
    return axis.map((item) => styleAxis(item, tokens));
  }
  return styleAxis(axis, tokens);
}

function styleAxis(axis: unknown, tokens: ChartTokens): Record<string, unknown> {
  const item = asRecord(axis);
  return {
    ...item,
    axisLine: { lineStyle: { color: tokens.line }, ...asRecord(item.axisLine) },
    axisLabel: { color: tokens.muted, ...asRecord(item.axisLabel) },
    splitLine: { show: true, lineStyle: { color: tokens.line }, ...asRecord(item.splitLine) },
  };
}
