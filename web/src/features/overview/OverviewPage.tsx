import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { NumericText } from "@/shared/ui/NumericText";
import { ArrowRight, Layers, RefreshCw, TrendingDown, TrendingUp } from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useParams } from "react-router";
import { getHistoryWindow, keys, listValuationPoints, unwrapOrRedirect } from "@/api";
import type { HistoryWindow } from "@/api";
import {
  formatAmount,
  formatCount,
  formatUsd,
  isZeroDecimal,
  parseDecimal,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { iconSize } from "@/shared/icons/iconSizes";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { HoldingsTree } from "@/features/holdings/pages/TreePage";
import { historyGranularity, type HistoryPreset } from "@/features/history/hooks/useHistoryWindow";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { CycleNotice } from "./components/CycleNotice";
import { HeroChart } from "./components/HeroChart";
import { isTerminalStatus, useCollectNow } from "./useCollectNow";
import { useOverview } from "./useOverview";
import styles from "@/features/overview/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function OverviewPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const query = useOverview(portfolioId);
  const collect = useCollectNow(portfolioId);
  const overview = query.data;
  const timezone = overview?.context.timezone;

  const [selectedRange, setSelectedRange] = useState<HistoryPreset>("3D");

  const rangeWindow = usePortfolioQuery<HistoryWindow>({
    key: (userId, portfolioId) => keys.overviewChartWindow(userId, portfolioId, selectedRange),
    ready: true,
    load: (portfolioId) =>
      getHistoryWindow({ path: { portfolioId }, query: { preset: selectedRange } }),
    placeholderData: keepPreviousData,
  });

  const rangePoints = useQuery({
    queryKey: keys.overviewChartPoints(
      userId,
      portfolioId,
      rangeWindow.data?.from,
      rangeWindow.data?.to,
    ),
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
      <section className={shared.wide}>
        <p className={shared.muted}>Loading the latest Valuation Point.</p>
      </section>
    );
  }

  const point = overview?.point ?? null;
  const boundaryText = point
    ? boundarySummary(point.boundaryContributionUsd, point.boundaryWithdrawalUsd)
    : null;

  return (
    <section className={shared.wide}>
      {collect.error ? (
        <p className={shared.alert} role="alert">
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
        <p className={shared.muted} role="status">
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
                      <NumericText>{formatAmount(overview.totalUsd)}</NumericText>
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
                <HeroChart
                  points={activePoints}
                  timezone={timezone}
                  preset={selectedRange}
                  onPreset={setSelectedRange}
                />
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
