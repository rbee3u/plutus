import { NumericText } from "@/shared/ui/NumericText";
import type { EChartsCoreOption } from "echarts/core";
import { ArrowRight, Layers, RefreshCw, TrendingDown, TrendingUp } from "lucide-react";
import { useMemo } from "react";
import { Link, useParams } from "react-router";
import type { CollectionCycle } from "@/api/generated/types.gen";
import {
  chartNumber,
  formatAmount,
  formatCount,
  formatUsd,
  isZeroDecimal,
  parseDecimal,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge, StatusBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { HoldingsTree } from "@/features/holdings/TreePage";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { useCollectNow, isTerminalStatus } from "@/features/overview/useCollectNow";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";

export function OverviewPage() {
  const { portfolioId = "" } = useParams();
  const query = useOverview(portfolioId);
  const collect = useCollectNow(portfolioId);
  const overview = query.data;
  const timezone = overview?.context.timezone;

  const sparklineOption = useMemo((): EChartsCoreOption | null => {
    const points = overview?.sparkline ?? [];
    if (points.length < 2) {
      return null;
    }
    return {
      grid: {
        left: 8,
        right: 12,
        top: 20,
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
        },
      },
      yAxis: {
        type: "value",
        scale: true,
        splitLine: {
          lineStyle: {
            opacity: 0.25,
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
          data: points.map((point) => chartNumber(point.totalUsd)),
          areaStyle: {
            opacity: 0.08,
          },
        },
      ],
    };
  }, [overview, timezone]);

  const changeInsight = useMemo(() => {
    const points = overview?.sparkline ?? [];
    if (points.length < 2 || !overview?.totalUsd) return null;
    const current = parseDecimal(overview.totalUsd);
    if (!current) return null;

    const latestTime = new Date(points[points.length - 1]?.evaluatedAt ?? 0).getTime();
    const target24h = latestTime - 24 * 60 * 60 * 1000;

    let baselinePoint = points[0];
    let minDiff = Infinity;
    for (const pt of points) {
      const ptTime = new Date(pt.evaluatedAt).getTime();
      const diff = Math.abs(ptTime - target24h);
      if (diff < minDiff) {
        minDiff = diff;
        baselinePoint = pt;
      }
    }

    if (!baselinePoint || baselinePoint === points[points.length - 1]) {
      baselinePoint = points[0];
    }

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
      label: points.length > 2 ? "24h" : "cycle",
    };
  }, [overview]);

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
            <article className={styles.heroCard}>
              <div className={styles.heroHeader}>
                <div className={styles.heroMeta}>
                  <h2 className={styles.heroTitle}>Total Net Worth</h2>
                  <span
                    className={`${styles.heroAccountsBadge} ${
                      overview.staleAccountCount > 0 || overview.incompleteAccountCount > 0
                        ? styles.heroAccountsBadgeAlert
                        : ""
                    }`}
                  >
                    {formatCount(overview.accountCount)} accounts
                    {overview.staleAccountCount > 0
                      ? ` · ${formatCount(overview.staleAccountCount)} stale`
                      : ""}
                    {overview.incompleteAccountCount > 0
                      ? ` · ${formatCount(overview.incompleteAccountCount)} incomplete`
                      : ""}
                  </span>
                </div>
                <div className={styles.heroSync}>
                  <span className={styles.heroUpdatedTime}>
                    Updated {formatInstant(point.evaluatedAt, timezone)}
                  </span>
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
                        {changeInsight.isPositive ? "+" : changeInsight.isNegative ? "-" : ""}$
                        {changeInsight.diffUsd} (
                        {changeInsight.isPositive ? "+" : changeInsight.isNegative ? "-" : ""}
                        {changeInsight.percent}%)
                        <span className={styles.changeLabel}>{changeInsight.label}</span>
                      </span>
                    ) : null}
                    <CompletenessBadge value={overview.completeness} />
                  </div>
                </div>
                {boundaryText ? (
                  <p className={styles.heroBoundaryNotice}>
                    <NumericText>{boundaryText}</NumericText>
                  </p>
                ) : null}
              </div>
            </article>
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
              <ChartBlock
                title="Recent Valuation Points"
                option={sparklineOption}
                empty="Fewer than two Valuation Points so far. Each Collection Cycle adds one."
              />
              <HoldingsTree portfolioId={portfolioId} tree={overview.tree} />
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
