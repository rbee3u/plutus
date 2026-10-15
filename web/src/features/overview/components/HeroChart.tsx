import { useChart } from "@/shared/lib/useChart";
import type { EChartsCoreOption } from "echarts/core";
import { useMemo } from "react";
import type { ValuationPointSummary } from "@/api";
import { chartNumber, formatAmount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { HISTORY_PRESETS, type HistoryPreset } from "@/features/history/timeWindow";
import styles from "@/features/overview/styles.module.css";

/** The recent valuation trend of the hero card, with its time-range presets. */
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

  const chartHostRef = useChart(sparklineOption);

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
          aria-label="Recent valuation points"
        />
      ) : (
        <div className={styles.heroChartEmpty}>
          At least two valuation points are needed to show a trend.
        </div>
      )}
    </div>
  );
}
