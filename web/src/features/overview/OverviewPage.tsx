import { NumericText } from "@/shared/ui/NumericText";
import type { EChartsCoreOption } from "echarts/core";
import { useMemo } from "react";
import { useParams } from "react-router";
import type { CollectionCycle, GroupTotal, Overview } from "@/api/generated/types.gen";
import {
  chartNumber,
  formatAmount,
  formatUsd,
  isZeroDecimal,
  formatCount,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { NO_POINT } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge, StatusBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
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
      xAxis: {
        type: "category",
        data: points.map((point) => formatInstant(point.evaluatedAt, timezone)),
      },
      yAxis: { type: "value", scale: true },
      series: [
        {
          type: "line",
          name: "Total value",
          data: points.map((point) => chartNumber(point.totalUsd)),
          tooltip: {
            valueFormatter: (_value: unknown, index: number) =>
              formatAmount(points[index]?.totalUsd),
          },
        },
      ],
    };
  }, [overview, timezone]);

  if (query.isPending && !overview) {
    return (
      <section className={styles.wide}>
        <p className={styles.muted}>Loading the latest Valuation Point.</p>
      </section>
    );
  }

  const point = overview?.point ?? null;

  return (
    <section className={styles.wide}>
      <div className={styles.headingRow}>
        <p className={styles.lead}>
          The latest Valuation Point across every enabled Identity. Unknown values stay blank, never
          zero.
        </p>
        <ActionButton
          type="button"
          tone="primary"
          disabled={
            collect.busy || Boolean(collect.cycle && !isTerminalStatus(collect.cycle.status))
          }
          onClick={() => void collect.start()}
        >
          Collect now
        </ActionButton>
      </div>
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
      {overview ? (
        <>
          {point ? (
            <div className={styles.cards}>
              <article className={styles.metric}>
                <h2>Total value</h2>
                <p className={styles.metricValue}>
                  <NumericText>{formatAmount(overview.totalUsd)}</NumericText>
                  {overview.totalUsd ? <span className={styles.unit}>USD</span> : null}
                </p>
                <p className={styles.metricSub}>
                  <CompletenessBadge value={overview.completeness} />
                </p>
              </article>
              <article className={styles.metric}>
                <h2>Accounts</h2>
                <p className={styles.metricValue}>{formatCount(overview.accountCount)}</p>
                <p className={styles.metricSub}>{accountNote(overview)}</p>
              </article>
              <article className={styles.metric}>
                <h2>Last point</h2>
                <p className={styles.metricValue}>{formatInstant(point.evaluatedAt, timezone)}</p>
                <p className={styles.metricSub}>
                  <span title={point.id}>
                    Point {shortId(point.id)} · {overview.context.timezone}
                  </span>
                </p>
                <p className={styles.metricSub}>
                  <NumericText>
                    {boundarySummary(point.boundaryContributionUsd, point.boundaryWithdrawalUsd)}
                  </NumericText>
                </p>
              </article>
            </div>
          ) : (
            <p className={styles.notice}>{NO_POINT}</p>
          )}
          {point ? (
            <>
              <div className={styles.panelGrid}>
                <section className={styles.panel}>
                  <h2>By platform</h2>
                  <DataTable
                    caption="Value per Platform at the latest point"
                    columns={GROUP_COLUMNS}
                    rows={overview.byPlatform.map(groupRow)}
                    empty="No Platform totals yet. Totals appear after the first Collection Cycle."
                  />
                </section>
                <section className={styles.panel}>
                  <h2>By identity</h2>
                  <DataTable
                    caption="Value per Identity at the latest point"
                    columns={GROUP_COLUMNS}
                    rows={overview.byIdentity.map(groupRow)}
                    empty="No Identity totals yet. Add an Identity, enable its connection, and collect."
                  />
                </section>
              </div>
              <ChartBlock
                title="Recent Valuation Points"
                summary="Portfolio total at each recent Valuation Point, incomplete points included as reported. History lists every point."
                option={sparklineOption}
                empty="Fewer than two Valuation Points so far. Each Collection Cycle adds one."
              />
            </>
          ) : null}
          {point ? (
            <IssuesPanel
              issues={overview.issues}
              empty="No issues at the latest Valuation Point."
            />
          ) : null}
        </>
      ) : null}
    </section>
  );
}

const GROUP_COLUMNS = [
  { key: "label", header: "Group" },
  { key: "valueUsd", header: "Value (USD)", numeric: true },
  { key: "accountCount", header: "Accounts", numeric: true },
  { key: "completeness", header: "Completeness" },
];

function groupRow(group: GroupTotal): DataRow {
  return {
    id: group.key,
    label: group.label,
    valueUsd: formatAmount(group.valueUsd),
    accountCount: formatCount(group.accountCount),
    completeness: <CompletenessBadge value={group.completeness} />,
  };
}

/** What is off about the accounts, or where they stand when nothing is. */
function accountNote(overview: Overview): string {
  const parts: string[] = [];
  if (overview.staleAccountCount > 0)
    parts.push(`${formatCount(overview.staleAccountCount)} stale`);
  if (overview.incompleteAccountCount > 0) {
    parts.push(`${formatCount(overview.incompleteAccountCount)} incomplete`);
  }
  return parts.length > 0 ? parts.join(" · ") : "inside the boundary";
}

function boundarySummary(contribution: string, withdrawal: string): string {
  const parts: string[] = [];
  if (!isZeroDecimal(contribution)) {
    parts.push(`boundary contribution ${formatUsd(contribution)}`);
  }
  if (!isZeroDecimal(withdrawal)) {
    parts.push(`boundary withdrawal ${formatUsd(withdrawal)}`);
  }
  return parts.length > 0 ? parts.join(" · ") : "No Boundary Events at this point.";
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
