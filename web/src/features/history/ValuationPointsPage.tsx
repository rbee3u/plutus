import { useQuery } from "@tanstack/react-query";
import type { EChartsCoreOption } from "echarts/core";
import { ChevronRight } from "lucide-react";
import { useMemo } from "react";
import { useParams } from "react-router";
import { getValuationPoint, listPeriodCloses, listValuationPoints } from "@/api/generated/sdk.gen";
import type {
  AccountValuation,
  BoundaryEvent,
  PeriodKind,
  ValuationPointDetail,
  Platform,
  ValuationPointSummary,
} from "@/api/generated/types.gen";
import { chartNumber, formatAmount, formatBoundaryFlows, isZeroDecimal } from "@/shared/lib/amount";
import { formatInstant, isoFromLocalInput } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { readChoice, useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { accountSummary, capitalize, reasonText } from "@/shared/lib/text";
import { CompletenessBadge, FreshnessBadge, StatusBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import { IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import styles from "@/features/pages.module.css";

const GRANULARITIES = ["raw", "hour", "day"] as const;
export const PERIOD_KINDS: readonly PeriodKind[] = ["day", "week", "month", "quarter", "year"];
const NO_POINT =
  "No Valuation Point in this window. Enable a connection and run a Collection Cycle.";
const POINT_DETAIL_ID = "valuation-point-detail";

export function ValuationPointsPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const { params, update } = useSearchState();
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const granularity = readChoice(params.get("granularity"), GRANULARITIES, "raw");
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, "month");
  const pointId = params.get("point") ?? "";

  const points = useQuery({
    queryKey: ["valuation-points", userId, portfolioId, from, to, granularity],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listValuationPoints({
          path: { portfolioId },
          query: { from: isoFromLocalInput(from), to: isoFromLocalInput(to), granularity },
        }),
        redirect,
      ),
  });
  const detail = useQuery({
    queryKey: ["valuation-point", userId, portfolioId, pointId],
    enabled: Boolean(userId && portfolioId && pointId),
    queryFn: async () =>
      unwrapOrRedirect(await getValuationPoint({ path: { portfolioId, pointId } }), redirect),
  });
  const closes = useQuery({
    queryKey: ["period-closes", userId, portfolioId, periodKind],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listPeriodCloses({ path: { portfolioId }, query: { periodKind } }),
        redirect,
      ),
  });
  const series = points.data;
  const timezone = series?.context.timezone;

  const chartOption = useMemo((): EChartsCoreOption | null => {
    const items = series?.items ?? [];
    if (items.length < 2) {
      return null;
    }
    return {
      xAxis: {
        type: "category",
        data: items.map((item) => formatInstant(item.evaluatedAt, timezone)),
      },
      yAxis: { type: "value", scale: true },
      series: [
        {
          type: "line",
          name: "Total value",
          data: items.map((item) => chartNumber(item.totalUsd)),
        },
      ],
    };
  }, [series, timezone]);

  return (
    <section className={styles.wide}>
      <p className={styles.lead}>
        One portfolio total per Collection Cycle. An empty window shows the last seven days; hour
        and day granularity keep the last point of each bucket.
      </p>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>From</span>
          <TextInput
            type="datetime-local"
            value={from}
            onChange={(event) => update({ from: event.target.value })}
          />
        </label>
        <label className={styles.field}>
          <span>To</span>
          <TextInput
            type="datetime-local"
            value={to}
            onChange={(event) => update({ to: event.target.value })}
          />
        </label>
        <label className={styles.field}>
          <span>Granularity</span>
          <Select
            value={granularity}
            onChange={(event) => update({ granularity: event.target.value })}
          >
            {GRANULARITIES.map((option) => (
              <option key={option} value={option}>
                {capitalize(option)}
              </option>
            ))}
          </Select>
        </label>
      </form>
      <QueryRecovery
        error={points.error}
        hasData={Boolean(series)}
        busy={points.isFetching}
        fallback="Valuation Points could not be loaded."
        onRetry={() => {
          void points.refetch();
        }}
      />
      {points.isPending && !series ? (
        <p className={styles.muted}>Loading Valuation Points.</p>
      ) : null}
      {series ? (
        <>
          <p className={styles.muted}>
            {formatInstant(series.from, timezone)} – {formatInstant(series.to, timezone)} in{" "}
            {series.context.timezone}, {series.granularity} granularity,{" "}
            {String(series.items.length)} {series.items.length === 1 ? "point" : "points"}.
          </p>
          <ChartBlock
            title="Portfolio value"
            summary="Total value at each Valuation Point in the window. Open a point to see its accounts and Boundary Events."
            option={chartOption}
            empty={
              series.items.length === 0
                ? NO_POINT
                : "Only one Valuation Point in this window; the chart needs two."
            }
            emptyRows={NO_POINT}
            rowClassName={(row) => (row.id === pointId ? styles.openRow : undefined)}
            openKey={pointId}
            onRowClick={(key) => update({ point: key === pointId ? "" : key })}
            detailId={POINT_DETAIL_ID}
            renderDetail={() => (
              <>
                <QueryRecovery
                  error={detail.error}
                  hasData={Boolean(detail.data)}
                  busy={detail.isFetching}
                  fallback="The Valuation Point could not be loaded."
                  onRetry={() => {
                    void detail.refetch();
                  }}
                />
                {detail.data ? <PointDetail detail={detail.data} timezone={timezone} /> : null}
                {detail.isPending ? <p className={styles.muted}>Loading the point.</p> : null}
              </>
            )}
            columns={[
              { key: "evaluatedAt", header: "Evaluated at", nowrap: true },
              { key: "totalUsd", header: "Total (USD)", numeric: true },
              { key: "completeness", header: "Completeness" },
              { key: "accounts", header: "Accounts" },
              { key: "boundary", header: "Boundary (USD)", numeric: true },
              { key: "actions", header: "" },
            ]}
            rows={series.items.map((item) => pointRow(item, timezone, pointId, update))}
          />
        </>
      ) : null}
      <section className={styles.panel}>
        <div className={styles.headingRow}>
          <h2>Period closes</h2>
          <label className={`${styles.field} ${styles.toolbarField}`}>
            <span>Period</span>
            <Select
              value={periodKind}
              onChange={(event) => update({ periodKind: event.target.value })}
            >
              {PERIOD_KINDS.map((option) => (
                <option key={option} value={option}>
                  {capitalize(option)}
                </option>
              ))}
            </Select>
          </label>
        </div>
        <p className={styles.muted}>
          The last Valuation Point at or before the end of each period. A period without a point
          says so instead of carrying a value.
        </p>
        <QueryRecovery
          error={closes.error}
          hasData={Boolean(closes.data)}
          busy={closes.isFetching}
          fallback="Period closes could not be loaded."
          onRetry={() => {
            void closes.refetch();
          }}
        />
        <DataTable
          caption={`Period closes per ${periodKind}`}
          columns={[
            { key: "label", header: "Period" },
            { key: "totalUsd", header: "Close (USD)", numeric: true },
            { key: "evaluatedAt", header: "Point evaluated", nowrap: true },
            { key: "completeness", header: "Completeness" },
            { key: "reasons", header: "Missing" },
          ]}
          rows={(closes.data?.items ?? []).map((item) => ({
            id: `${item.periodKind}-${item.periodStart}`,
            label: (
              <span className={styles.cellStack}>
                <span>{item.label}</span>
                <span className={`${styles.cellMeta} ${styles.nowrap}`}>
                  {formatInstant(item.periodStart, timezone)} –{" "}
                  {formatInstant(item.periodEnd, timezone)}
                </span>
              </span>
            ),
            totalUsd: item.point ? (
              formatAmount(item.point.totalUsd)
            ) : (
              <span className={styles.cellMeta}>no point</span>
            ),
            evaluatedAt: item.point ? formatInstant(item.point.evaluatedAt, timezone) : "—",
            completeness: <CompletenessBadge value={item.completeness} />,
            reasons: item.reasons.length > 0 ? item.reasons.map(reasonText).join(", ") : "—",
          }))}
          empty={
            closes.isPending
              ? "Loading period closes."
              : "No periods yet. Periods appear once the Portfolio has a Valuation Point."
          }
        />
      </section>
    </section>
  );
}

function pointRow(
  item: ValuationPointSummary,
  timezone: string | undefined,
  selectedId: string,
  update: (next: Record<string, string>) => void,
): DataRow {
  const open = selectedId === item.id;
  const evaluatedAt = formatInstant(item.evaluatedAt, timezone);
  return {
    id: item.id,
    evaluatedAt,
    totalUsd: formatAmount(item.totalUsd),
    completeness: <CompletenessBadge value={item.completeness} />,
    accounts: accountSummary(
      item.accountCount,
      item.staleAccountCount,
      item.incompleteAccountCount,
    ),
    // A known zero on both sides is one quiet figure, not a signed pair.
    boundary:
      isZeroDecimal(item.boundaryContributionUsd) && isZeroDecimal(item.boundaryWithdrawalUsd) ? (
        <span className={styles.cellMeta}>0.00</span>
      ) : (
        `+${formatAmount(item.boundaryContributionUsd)} / −${formatAmount(item.boundaryWithdrawalUsd)}`
      ),
    actions: (
      <span className={styles.rowActions}>
        <IconButton
          label={`${open ? "Collapse" : "Expand"} the Valuation Point at ${evaluatedAt}`}
          aria-expanded={open}
          aria-controls={open ? POINT_DETAIL_ID : undefined}
          onClick={() => update({ point: open ? "" : item.id })}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? styles.chevronOpen : styles.chevron}
          />
        </IconButton>
      </span>
    ),
  };
}

export const ACCOUNT_VALUATION_COLUMNS = [
  { key: "account", header: "Account" },
  { key: "platform", header: "Platform" },
  { key: "valueUsd", header: "Value (USD)", numeric: true },
  { key: "method", header: "Method" },
  { key: "freshness", header: "Freshness" },
  { key: "completeness", header: "Completeness" },
  { key: "status", header: "Status" },
  { key: "error", header: "Error" },
  { key: "reportObservedAt", header: "Report observed", nowrap: true },
];

export function accountValuationRow(
  item: AccountValuation,
  timezone?: string,
  platforms?: Platform[],
): DataRow {
  return {
    id: item.accountId,
    account: (
      <span className={styles.cellStack}>
        <span>{item.accountName}</span>
        <span className={styles.cellMeta}>{item.identityName}</span>
      </span>
    ),
    platform: platformLabel(platforms, item.platform),
    valueUsd: formatAmount(item.valueUsd),
    method: item.method,
    freshness: <FreshnessBadge value={item.freshness} />,
    completeness: <CompletenessBadge value={item.completeness} />,
    status: <StatusBadge value={item.status} />,
    error: item.errorClass
      ? `${item.errorClass}${item.errorDetail ? `: ${item.errorDetail}` : ""}`
      : "—",
    reportObservedAt: formatInstant(item.reportObservedAt, timezone),
  };
}

export const BOUNDARY_COLUMNS = [
  { key: "evaluatedAt", header: "Evaluated at", nowrap: true },
  { key: "account", header: "Account" },
  { key: "platform", header: "Platform" },
  { key: "direction", header: "Direction" },
  { key: "valueUsd", header: "Value (USD)", numeric: true },
  { key: "method", header: "Method" },
];

export function boundaryEventRow(
  item: BoundaryEvent,
  timezone?: string,
  platforms?: Platform[],
): DataRow {
  return {
    id: `${item.pointId}-${item.accountId}-${item.direction}`,
    evaluatedAt: formatInstant(item.evaluatedAt, timezone),
    account: (
      <span className={styles.cellStack}>
        <span>{item.accountName}</span>
        {item.identityName ? <span className={styles.cellMeta}>{item.identityName}</span> : null}
      </span>
    ),
    platform: item.platform ? platformLabel(platforms, item.platform) : "—",
    direction: capitalize(item.direction),
    valueUsd: formatAmount(item.valueUsd),
    method: item.method,
  };
}

function PointDetail({ detail, timezone }: { detail: ValuationPointDetail; timezone?: string }) {
  const platforms = usePlatforms();
  const point = detail.point;
  return (
    <>
      <dl className={styles.metaList}>
        <div>
          <dt>Boundary flows</dt>
          <dd>{formatBoundaryFlows(point)}</dd>
        </div>
        <div>
          <dt>Collection Cycle</dt>
          <dd title={point.jobId}>{shortId(point.jobId)}</dd>
        </div>
        {point.recalculatedAt ? (
          <div>
            <dt>Recalculated</dt>
            <dd>{formatInstant(point.recalculatedAt, timezone)}</dd>
          </div>
        ) : null}
      </dl>
      <h3>Accounts</h3>
      <DataTable
        caption="Account valuations at this point"
        columns={ACCOUNT_VALUATION_COLUMNS}
        rows={detail.accounts.map((item) =>
          accountValuationRow(item, timezone, platforms.data?.items),
        )}
        empty="No accounts were inside the boundary at this point."
      />
      <h3>Boundary Events</h3>
      <DataTable
        caption="Boundary Events at this point"
        columns={BOUNDARY_COLUMNS}
        rows={detail.boundaryEvents.map((item) =>
          boundaryEventRow(item, timezone, platforms.data?.items),
        )}
        empty="No account entered or left the boundary at this point."
      />
      {detail.issues.length > 0 ? (
        <>
          <h3>Issues</h3>
          <IssueList issues={detail.issues} />
        </>
      ) : null}
    </>
  );
}
