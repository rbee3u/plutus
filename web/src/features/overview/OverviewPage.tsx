import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { NumericText } from "@/shared/ui/NumericText";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { ArrowRight, Layers, RefreshCw, TrendingDown, TrendingUp } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router";
import { getHistoryWindow, listValuationPoints } from "@/api/generated/sdk.gen";
import type { CollectionCycle } from "@/api/generated/types.gen";
import {
  chartNumber,
  formatAmount,
  formatCount,
  formatUsd,
  isZeroDecimal,
  parseDecimal,
} from "@/shared/lib/amount";
import { withChartTheme } from "@/shared/lib/chartTheme";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { iconSize } from "@/shared/icons/iconSizes";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge, StatusBadge } from "@/shared/ui/Badge";
import { HoldingsTree } from "@/features/holdings/TreePage";
import {
  HISTORY_PRESETS,
  historyGranularity,
  type HistoryPreset,
} from "@/features/history/useHistoryWindow";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { useCollectNow, isTerminalStatus } from "@/features/overview/useCollectNow";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";

export function OverviewPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const query = useOverview(portfolioId);
  const collect = useCollectNow(portfolioId);
  const overview = query.data;
  const timezone = overview?.context.timezone;

  const [selectedRange, setSelectedRange] = useState<HistoryPreset>("3D");

  const rangeWindow = useQuery({
    queryKey: ["overview-chart-window", userId, portfolioId, selectedRange],
    enabled: Boolean(userId && portfolioId),
    placeholderData: keepPreviousData,
    queryFn: async () =>
      unwrapOrRedirect(
        await getHistoryWindow({
          path: { portfolioId },
          query: { preset: selectedRange },
        }),
        redirect,
      ),
  });

  const rangePoints = useQuery({
    queryKey: [
      "overview-chart-points",
      userId,
      portfolioId,
      rangeWindow.data?.from,
      rangeWindow.data?.to,
    ],
    enabled: Boolean(userId && portfolioId && rangeWindow.data),
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const from = rangeWindow.data!.from;
      const to = rangeWindow.data!.to;
      const granularity = historyGranularity(from, to);
      return unwrapOrRedirect(
        await listValuationPoints({
          path: { portfolioId },
          query: { from, to, granularity },
        }),
        redirect,
      );
    },
  });

  const activePoints = useMemo(
    () => rangePoints.data?.items ?? overview?.sparkline ?? [],
    [rangePoints.data?.items, overview?.sparkline],
  );

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
    const points = activePoints;
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
        valueFormatter: (_value: unknown, index: number) =>
          `${formatAmount(points[index]?.totalUsd)} USD`,
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
  }, [activePoints, timezone]);

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

  const changeInsight = useMemo(() => {
    const points = activePoints;
    if (points.length < 2 || !overview?.totalUsd) return null;
    const current = parseDecimal(overview.totalUsd);
    if (!current) return null;

    const baselinePoint = points[0];
    const baseline = parseDecimal(baselinePoint?.totalUsd);
    if (!baseline || baseline.isZero()) return null;

    const diff = current.minus(baseline);
    const percent = diff.dividedBy(baseline).times(100);
    const isPositive = diff.gt(0);
    const isNegative = diff.lt(0);
    const isZero = diff.isZero();

    return {
      diffUsd: formatAmount(diff.abs().toFixed()),
      percent: percent.abs().toFixed(2),
      isPositive,
      isNegative,
      isZero,
      label: selectedRange,
    };
  }, [activePoints, overview, selectedRange]);

  if (query.isPending && !overview) {
    return (
      <section className={styles.wide}>
        <p className={styles.muted}>Loading the latest Valuation Point.</p>
      </section>
    );
  }

  const point = overview?.point ?? null;
  const boundaryText = point
    ? boundarySummary(point.boundaryContributionUsd, point.boundaryWithdrawalUsd)
    : null;

  return (
    <section className={styles.wide}>
      {collect.error ? (
        <p className={styles.alert} role="alert">
          {collect.error}
        </p>
      ) : null}
      {collect.cycle ? <CycleNotice cycle={collect.cycle} timezone={timezone} /> : null}
      <QueryRecovery
        error={query.error}
        hasData={Boolean(overview)}
        busy={query.isFetching}
        fallback="Overview could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      {query.isFetching && overview ? (
        <p className={styles.muted} role="status">
          Updating the latest Valuation Point.
        </p>
      ) : null}
      {overview ? (
        <>
          {point ? (
            <div className={styles.heroContainer}>
              <article className={styles.heroCard}>
                <div className={styles.heroHeader}>
                  <div className={styles.heroMeta}>
                    <h2 className={styles.heroTitle}>Total Net Worth</h2>
                  </div>
                  <div className={styles.heroSync}>
                    <ActionButton
                      type="button"
                      tone="secondary"
                      disabled={
                        collect.busy ||
                        Boolean(collect.cycle && !isTerminalStatus(collect.cycle.status))
                      }
                      onClick={() => void collect.start()}
                      className={styles.syncButton}
                    >
                      <RefreshCw
                        size={iconSize.xs}
                        aria-hidden="true"
                        className={
                          collect.busy ||
                          Boolean(collect.cycle && !isTerminalStatus(collect.cycle.status))
                            ? styles.spin
                            : ""
                        }
                      />
                      Collect now
                    </ActionButton>
                  </div>
                </div>

                <div className={styles.heroBody}>
                  <div className={styles.heroAmountRow}>
                    <div className={styles.heroAmount}>
                      <span className={styles.heroCurrencySymbol}>$</span>
                      <NumericText>{formatAmount(overview.totalUsd)}</NumericText>{" "}
                      {overview.totalUsd ? <span className={styles.heroUnit}>USD</span> : null}
                    </div>
                    <div className={styles.heroBadges}>
                      {changeInsight ? (
                        <span
                          className={
                            changeInsight.isPositive
                              ? styles.changePositive
                              : changeInsight.isNegative
                                ? styles.changeNegative
                                : styles.changeNeutral
                          }
                        >
                          {changeInsight.isPositive ? (
                            <TrendingUp size={iconSize.xs} aria-hidden="true" />
                          ) : changeInsight.isNegative ? (
                            <TrendingDown size={iconSize.xs} aria-hidden="true" />
                          ) : null}
                          <NumericText>
                            {`${changeInsight.isPositive ? "+" : changeInsight.isNegative ? "-" : ""}$${changeInsight.diffUsd}`}
                          </NumericText>{" "}
                          ({changeInsight.isPositive ? "+" : changeInsight.isNegative ? "-" : ""}
                          {changeInsight.percent}%)
                          <span className={styles.changeLabel}>{changeInsight.label}</span>
                        </span>
                      ) : null}
                      <CompletenessBadge value={overview.completeness} />
                    </div>
                  </div>

                  <div className={styles.heroSubMeta}>
                    <span className={styles.heroUpdatedTime}>
                      Updated {formatInstant(point.evaluatedAt, timezone)}
                    </span>
                    <span className={styles.heroMetaDot}>·</span>
                    <span className={styles.heroAccountsCount}>
                      {formatCount(overview.accountCount)} accounts
                    </span>
                    {overview.staleAccountCount > 0 ? (
                      <span className={styles.heroAccountsBadgeAlert}>
                        {formatCount(overview.staleAccountCount)} stale
                      </span>
                    ) : null}
                    {overview.incompleteAccountCount > 0 ? (
                      <span className={styles.heroAccountsBadgeAlert}>
                        {formatCount(overview.incompleteAccountCount)} incomplete
                      </span>
                    ) : null}
                  </div>

                  {boundaryText ? (
                    <p className={styles.heroBoundaryNotice}>
                      <NumericText>{boundaryText}</NumericText>
                    </p>
                  ) : null}
                </div>
              </article>

              {overview.sparkline.length >= 2 || (rangePoints.data?.items?.length ?? 0) >= 2 ? (
                <div className={styles.heroChartWrapper}>
                  <div className={styles.heroChartHeader}>
                    <span className={styles.heroChartLabel}>Recent valuation trend</span>
                    <div
                      className={styles.chartPresets}
                      role="tablist"
                      aria-label="Chart time range"
                    >
                      {HISTORY_PRESETS.map((preset) => {
                        const active = selectedRange === preset;
                        return (
                          <button
                            key={preset}
                            type="button"
                            role="tab"
                            aria-selected={active}
                            className={active ? styles.chartPresetActive : styles.chartPreset}
                            onClick={() => setSelectedRange(preset)}
                          >
                            {preset}
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
                    <div className={styles.heroChartEmpty}>
                      No valuation points recorded for this period.
                    </div>
                  )}
                </div>
              ) : null}
            </div>
          ) : (
            <div className={styles.emptyHero}>
              <div className={styles.emptyHeroIcon}>
                <Layers size={iconSize.lg} aria-hidden="true" />
              </div>
              <h2 className={styles.emptyHeroTitle}>No Valuation Data Yet</h2>
              <p className={styles.emptyHeroText}>
                This portfolio has no valuation history. Connect an exchange identity in Settings,
                or run a collection cycle to record your first valuation point.
              </p>
              <div className={styles.emptyHeroActions}>
                <ActionButton
                  type="button"
                  tone="primary"
                  disabled={
                    collect.busy ||
                    Boolean(collect.cycle && !isTerminalStatus(collect.cycle.status))
                  }
                  onClick={() => void collect.start()}
                  className={styles.syncButton}
                >
                  <RefreshCw
                    size={iconSize.xs}
                    aria-hidden="true"
                    className={
                      collect.busy ||
                      Boolean(collect.cycle && !isTerminalStatus(collect.cycle.status))
                        ? styles.spin
                        : ""
                    }
                  />
                  Collect now
                </ActionButton>
                <Link to={`/p/${portfolioId}/identities`} className={styles.emptyHeroLink}>
                  Manage identities
                  <ArrowRight size={iconSize.xs} aria-hidden="true" />
                </Link>
              </div>
            </div>
          )}
          {point ? (
            <>
              <HoldingsTree portfolioId={portfolioId} tree={overview.tree} />
              <IssuesPanel
                issues={overview.tree.issues}
                empty="No issues at the latest Valuation Point."
              />
            </>
          ) : null}
          {point ? (
            <IssuesPanel
              issues={overview.tree.issues}
              empty="No issues at the latest Valuation Point."
            />
          ) : null}
        </>
      ) : null}
    </section>
  );
}

function boundarySummary(contribution: string, withdrawal: string): string | null {
  const parts: string[] = [];
  if (!isZeroDecimal(contribution)) {
    parts.push(`boundary contribution ${formatUsd(contribution)}`);
  }
  if (!isZeroDecimal(withdrawal)) {
    parts.push(`boundary withdrawal ${formatUsd(withdrawal)}`);
  }
  return parts.length > 0 ? parts.join(" · ") : null;
}

function CycleNotice({ cycle, timezone }: { cycle: CollectionCycle; timezone?: string }) {
  const finished = isTerminalStatus(cycle.status);
  return (
    <p className={cycle.status === "failed" ? styles.alert : styles.notice} role="status">
      Collection Cycle {shortId(cycle.id)} <StatusBadge value={cycle.status} />{" "}
      {finished
        ? `at ${formatInstant(cycle.finishedAt, timezone)}.`
        : "· this page refreshes when it finishes."}
      <NumericText>
        {cycle.point
          ? ` Point total ${formatUsd(cycle.point.totalUsd)} (${cycle.point.completeness}).`
          : ""}
      </NumericText>
      {cycle.lastErrorClass
        ? ` ${cycle.lastErrorClass}${cycle.lastDetail ? `: ${cycle.lastDetail}` : ""}`
        : ""}
    </p>
  );
}
