import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { useEffect, useMemo, useRef, useState } from "react";
import type { ValuationPointSummary } from "@/api";
import { chartNumber, formatAmount } from "@/shared/lib/amount";
import { withChartTheme } from "@/shared/lib/chartTheme";
import { formatInstant } from "@/shared/lib/datetime";
import { HISTORY_PRESETS, type HistoryPreset } from "@/features/history/hooks/useHistoryWindow";
import styles from "@/features/overview/styles.module.css";

/** The recent valuation trend of the Hero card, with its time-range presets. */
export function HeroChart({
  points,
  timezone,
  preset,
  onPreset,
}: {
  points: ValuationPointSummary[];
  timezone?: string;
  preset: HistoryPreset;
  onPreset: (preset: HistoryPreset) => void;
}) {
  const chartHostRef = useRef<HTMLDivElement>(null);
  const [themeTick, setThemeTick] = useState(0);

  useEffect(() => {
    const root = document.documentElement;
    const observer = new MutationObserver(() => setThemeTick((value) => value + 1));
    observer.observe(root, {
      attributes: true,
      attributeFilter: ["data-resolved-theme", "data-theme"],
    });
    return () => observer.disconnect();
  }, []);

  const sparklineOption = useMemo((): EChartsCoreOption | null => {
    if (points.length < 2) {
      return null;
    }
    return {
      grid: {
        left: 12,
        right: 16,
        top: 16,
        bottom: 24,
        containLabel: true,
      },
      tooltip: {
        trigger: "axis",
        valueFormatter: (_value: unknown, index: number) => {
          const amt = formatAmount(points[index]?.totalUsd);
          return amt === "—" ? amt : `$${amt}`;
        },
      },
      xAxis: {
        type: "category",
        boundaryGap: false,
        data: points.map((point) => formatInstant(point.evaluatedAt, timezone)),
        axisLabel: {
          hideOverlap: true,
          showMaxLabel: true,
          showMinLabel: true,
          fontSize: 11,
        },
      },
      yAxis: {
        type: "value",
        scale: true,
        splitLine: {
          lineStyle: {
            opacity: 0.15,
          },
        },
        axisLabel: {
          formatter: (value: number) => {
            if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
            if (value >= 1_000) return `${(value / 1_000).toFixed(0)}K`;
            return `${value}`;
          },
        },
      },
      series: [
        {
          type: "line",
          name: "Total value",
          smooth: true,
          showSymbol: false,
          lineStyle: {
            width: 2.5,
          },
          data: points.map((point) => chartNumber(point.totalUsd)),
          areaStyle: {
            opacity: 0.1,
          },
        },
      ],
    };
  }, [points, timezone]);

  useEffect(() => {
    const host = chartHostRef.current;
    if (!host || !sparklineOption) {
      return;
    }
    const chart = echarts.init(host);
    chart.setOption(withChartTheme(sparklineOption), true);
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(host);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [sparklineOption, themeTick]);

  return (
    <div className={styles.heroChartWrapper}>
      <div className={styles.heroChartHeader}>
        <span className={styles.heroChartLabel}>Recent valuation trend</span>
        <div className={styles.chartPresets} role="tablist" aria-label="Chart time range">
          {HISTORY_PRESETS.map((presetOption) => {
            const active = preset === presetOption;
            return (
              <button
                key={presetOption}
                type="button"
                role="tab"
                aria-selected={active}
                className={active ? styles.chartPresetActive : styles.chartPreset}
                onClick={() => onPreset(presetOption)}
              >
                {presetOption}
              </button>
            );
          })}
        </div>
      </div>
      {sparklineOption ? (
        <div
          ref={chartHostRef}
          className={styles.heroChart}
          role="img"
          aria-label="Recent Valuation Points"
        />
      ) : (
        <div className={styles.heroChartEmpty}>No valuation points recorded for this period.</div>
      )}
    </div>
  );
}
