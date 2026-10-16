import { keepPreviousData } from "@tanstack/react-query";
import { ArrowRight, Layers, RefreshCw, TrendingDown, TrendingUp } from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useParams } from "react-router";
import { getHistoryWindow, keys, listValuationPoints } from "@/api";
import type { HistoryWindow } from "@/api";
import { historyGranularity, type HistoryPreset } from "@/shared/lib/timeWindow";
import { HoldingsTree } from "@/features/holdings/display";
import { iconSize } from "@/shared/icons/iconSizes";
import {
  formatAmount,
  formatCount,
  formatPercent,
  formatUsd,
  isZeroDecimal,
  parseDecimal,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { accountIssueScope } from "@/shared/lib/valuationRows";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { identitySetupGuidance, useIdentities } from "@/shared/lib/useIdentities";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { CycleNotice } from "../components/CycleNotice";
import { HeroChart } from "../components/HeroChart";
import { isTerminalStatus, useCollectNow } from "../hooks/useCollectNow";
import { useOverview } from "../hooks/useOverview";
import styles from "@/features/overview/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function OverviewPage() {
  const { portfolioId = "" } = useParams();
  const query = useOverview(portfolioId);
  const identities = useIdentities(portfolioId);
  const noIdentities = identities.isSuccess && identities.data.items.length === 0;
  const setup = identities.isSuccess ? identitySetupGuidance(identities.data.items) : null;
  const collect = useCollectNow(portfolioId);
  const collecting =
    collect.busy || Boolean(collect.cycle && !isTerminalStatus(collect.cycle.status));
  const overview = query.data;
  const timezone = overview?.context.timezone;

  const resolveIssueScope = useMemo(
    () =>
      accountIssueScope(
        overview?.tree.platforms.flatMap((platform) =>
          platform.identities.flatMap((identity) => identity.accounts),
        ),
        overview?.tree.platforms.map((platform) => ({
          id: platform.platform,
          name: platform.label,
        })),
      ),
    [overview?.tree],
  );

  const [selectedRange, setSelectedRange] = useState<HistoryPreset>("1W");

  const rangeWindow = usePortfolioQuery<HistoryWindow>({
    key: (userId, portfolioId) => keys.overviewChartWindow(userId, portfolioId, selectedRange),
    load: (portfolioId) =>
      getHistoryWindow({ path: { portfolioId }, query: { preset: selectedRange } }),
    placeholderData: keepPreviousData,
  });

  const rangePoints = usePortfolioQuery({
    key: (userId, portfolioId) =>
      keys.overviewChartPoints(userId, portfolioId, rangeWindow.data?.from, rangeWindow.data?.to),
    ready: Boolean(rangeWindow.data),
    placeholderData: keepPreviousData,
    load: (portfolioId) => {
      const from = rangeWindow.data!.from;
      const to = rangeWindow.data!.to;
      const granularity = historyGranularity(from, to);
      return listValuationPoints({
        path: { portfolioId },
        query: { from, to, granularity },
      });
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
    const percent = diff.dividedBy(baseline);
    const isPositive = diff.gt(0);
    const isNegative = diff.lt(0);

    return {
      diffUsd: formatAmount(diff.abs().toFixed()),
      percent: formatPercent(percent.abs().toFixed()),
      isPositive,
      isNegative,
      label: selectedRange,
    };
  }, [activePoints, overview, selectedRange]);

  if (query.isPending && !overview) {
    return (
      <section className={shared.wide}>
        <p className={shared.muted}>Loading the latest valuation point.</p>
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
      <QueryRecovery query={query} fallback="Overview could not be loaded." />
      <QueryRecovery query={identities} fallback="Identities could not be loaded." />
      {query.isFetching && overview ? (
        <p className={shared.muted} role="status">
          Updating the latest valuation point.
        </p>
      ) : null}
      {overview ? (
        <>
          {point && !noIdentities ? (
            <div className={styles.heroContainer}>
              <article className={styles.heroCard}>
                <div className={styles.heroHeader}>
                  <div className={styles.heroMeta}>
                    <h2 className={styles.heroTitle}>Total net worth</h2>
                  </div>
                  <div className={styles.heroSync}>
                    <ActionButton
                      type="button"
                      tone="secondary"
                      disabled={collecting}
                      onClick={() => void collect.start()}
                      className={styles.syncButton}
                    >
                      <RefreshCw
                        size={iconSize.xs}
                        aria-hidden="true"
                        className={collecting ? styles.spin : ""}
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
                          <NumericText>
                            {`(${changeInsight.isPositive ? "+" : changeInsight.isNegative ? "-" : ""}${changeInsight.percent})`}
                          </NumericText>
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
                      {formatCount(overview.accountCount)}{" "}
                      {overview.accountCount === 1 ? "account" : "accounts"}
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
              <h2 className={styles.emptyHeroTitle}>{setup?.title ?? "No valuation point yet"}</h2>
              <p className={styles.emptyHeroText}>
                {setup?.help ??
                  (identities.isSuccess
                    ? "Run a collection cycle to create the first valuation point."
                    : identities.isPending
                      ? "Loading identities."
                      : "Check the notice above to retry loading identities.")}
              </p>
              <div className={styles.emptyHeroActions}>
                {!setup ? (
                  <ActionButton
                    type="button"
                    tone="primary"
                    disabled={collecting || !identities.isSuccess}
                    onClick={() => void collect.start()}
                    className={styles.syncButton}
                  >
                    <RefreshCw
                      size={iconSize.xs}
                      aria-hidden="true"
                      className={collecting ? styles.spin : ""}
                    />
                    Collect now
                  </ActionButton>
                ) : null}
                <Link
                  to={`/p/${portfolioId}/data/identities`}
                  className={`${styles.emptyHeroLink} ${setup ? styles.emptyHeroPrimaryLink : ""}`}
                >
                  Manage identities
                  <ArrowRight size={iconSize.xs} aria-hidden="true" />
                </Link>
              </div>
            </div>
          )}
          <HoldingsTree portfolioId={portfolioId} tree={overview.tree} />
          {point ? (
            <IssuesPanel
              issues={overview.tree.issues}
              timezone={timezone}
              resolveScope={resolveIssueScope}
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
