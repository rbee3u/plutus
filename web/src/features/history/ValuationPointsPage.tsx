import { useParams } from "react-router";
import { NumericText } from "@/shared/ui/NumericText";
import { useQuery } from "@tanstack/react-query";
import type { EChartsCoreOption } from "echarts/core";
import { ChevronRight } from "lucide-react";
import { useMemo, useState, useEffect } from "react";
import {
  getValuationPoint,
  listPeriodCloses,
  listValuationPoints,
  listValuationPointPage,
} from "@/api/generated/sdk.gen";
import type {
  AccountValuation,
  BoundaryEvent,
  PeriodKind,
  ValuationPointDetail,
  Platform,
  ValuationPointSummary,
} from "@/api/generated/types.gen";
import {
  chartNumber,
  formatAmount,
  formatBoundaryFlows,
  isZeroDecimal,
  parseDecimal,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { readChoice } from "@/shared/lib/searchParams";
import { unwrapOrRedirect } from "@/shared/lib/session";
import { accountSummary, capitalize, reasonText } from "@/shared/lib/text";
import { CompletenessBadge, DirectionBadge, FreshnessBadge, StatusBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Select } from "@/shared/ui/FormControls";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import { IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { HistoricalPositions } from "@/features/holdings/HistoricalPositions";
import { HistoryFilters, HistoryPager } from "./HistoryFilters";
import { useHistoryWindow, useHistoryPager, historyGranularity } from "./useHistoryWindow";
import styles from "@/features/pages.module.css";

export const PERIOD_KINDS: readonly PeriodKind[] = ["day", "week", "month", "quarter", "year"];
const NO_POINT =
  "No Valuation Point in this window. Enable a connection and run a Collection Cycle.";
const POINT_DETAIL_ID = "valuation-point-detail";

export function ValuationPointsPage() {
  const history = useHistoryWindow();
  const { portfolioId, userId, redirect, params, update, window } = history;
  const from = window?.from;
  const to = window?.to;
  const granularity = from && to ? historyGranularity(from, to) : "hour";
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, history.defaultPeriod);
  const pointId = params.get("point") ?? "";
  const pointPager = useHistoryPager("points");

  const points = useQuery({
    queryKey: ["valuation-points", userId, portfolioId, from, to, granularity],
    enabled: Boolean(userId && portfolioId && window),
    queryFn: async () =>
      unwrapOrRedirect(
        await listValuationPoints({
          path: { portfolioId },
          query: { from, to, granularity },
        }),
        redirect,
      ),
  });
  const records = useQuery({
    queryKey: [
      "valuation-point-page",
      userId,
      portfolioId,
      from,
      to,
      pointPager.offset,
      pointPager.limit,
    ],
    enabled: Boolean(userId && portfolioId && window),
    queryFn: async () =>
      unwrapOrRedirect(
        await listValuationPointPage({
          path: { portfolioId },
          query: { from, to, offset: pointPager.offset, limit: pointPager.limit },
        }),
        redirect,
      ),
  });
  useEffect(() => {
    const total = records.data?.page.total;
    if (total !== undefined && pointPager.offset > 0 && pointPager.offset >= total) {
      pointPager.onChange(Math.max(0, Math.ceil(total / pointPager.limit) - 1) * pointPager.limit);
    }
  }, [records.data, pointPager]);
  const detail = useQuery({
    queryKey: ["valuation-point", userId, portfolioId, pointId],
    enabled: Boolean(userId && portfolioId && pointId),
    queryFn: async () =>
      unwrapOrRedirect(await getValuationPoint({ path: { portfolioId, pointId } }), redirect),
  });
  const closes = useQuery({
    queryKey: ["period-closes", userId, portfolioId, periodKind, from, to],
    enabled: Boolean(userId && portfolioId && window),
    queryFn: async () =>
      unwrapOrRedirect(
        await listPeriodCloses({ path: { portfolioId }, query: { periodKind, from, to } }),
        redirect,
      ),
  });
  const series = points.data;
  const timezone = window?.context.timezone;
  const closeItems = [...(closes.data?.items ?? [])].reverse();
  const closePager = useHistoryPager("closes", closeItems.length);

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
          tooltip: {
            valueFormatter: (_value: unknown, index: number) =>
              formatAmount(items[index]?.totalUsd),
          },
        },
      ],
    };
  }, [series, timezone]);

  return (
    <section className={styles.wide}>
      <HistoryFilters state={history} />
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
          <p className={styles.historyMetaNotice}>
            <span>
              {formatInstant(series.from, timezone)} – {formatInstant(series.to, timezone)}
            </span>
            <span className={styles.bullet}>·</span>
            <span>{series.context.timezone}</span>
            <span className={styles.bullet}>·</span>
            <span>{series.granularity} granularity</span>
          </p>
          <ChartBlock
            title="Portfolio value"
            option={chartOption}
            empty={
              series.items.length === 0
                ? NO_POINT
                : "Only one Valuation Point in this window; the chart needs two."
            }
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
              onChange={(event) => update({ periodKind: event.target.value, closesOffset: "" })}
            >
              {PERIOD_KINDS.map((option) => (
                <option key={option} value={option}>
                  {capitalize(option)}
                </option>
              ))}
            </Select>
          </label>
        </div>
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
          rows={closeItems
            .slice(closePager.offset, closePager.offset + closePager.limit)
            .map((item) => ({
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
              completeness: item.point ? <CompletenessBadge value={item.completeness} /> : "—",
              reasons: item.reasons.length > 0 ? item.reasons.map(reasonText).join(", ") : "—",
            }))}
          empty={
            closes.isPending
              ? "Loading period closes."
              : "No periods yet. Periods appear once the Portfolio has a Valuation Point."
          }
        />
        <HistoryPager
          label="Period closes"
          pager={closePager}
          total={closeItems.length}
          busy={closes.isFetching}
        />
      </section>
      <section className={styles.panel} aria-labelledby="raw-points-heading">
        <h2 id="raw-points-heading">Raw Valuation Points</h2>
        <QueryRecovery
          error={records.error}
          hasData={Boolean(records.data)}
          busy={records.isFetching}
          fallback="Raw Valuation Points could not be loaded."
          onRetry={() => {
            void records.refetch();
          }}
        />
        <DataTable
          caption="Raw Valuation Points"
          empty={records.isPending ? "Loading raw Valuation Points." : NO_POINT}
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
          rows={(records.data?.items ?? []).map((item) =>
            pointRow(item, timezone, pointId, update),
          )}
        />
        <HistoryPager
          label="Raw Valuation Points"
          pager={pointPager}
          total={records.data?.page.total ?? 0}
          busy={records.isFetching}
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
        <span className={styles.cellMeta}>{formatAmount(item.boundaryContributionUsd)}</span>
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
    status: <StatusBadge value={item.status} silentWhenHealthy />,
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
  const dec = parseDecimal(item.valueUsd);
  const isContribution = item.direction.toLowerCase() === "contribution";
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
    direction: <DirectionBadge value={item.direction} />,
    valueUsd: dec ? (
      <span className={isContribution ? styles.pnlPositive : styles.pnlNegative}>
        <NumericText>{`${isContribution ? "+" : "-"}$${formatAmount(dec.abs().toFixed())}`}</NumericText>
      </span>
    ) : (
      <span className={styles.muted}>—</span>
    ),
    method: <StatusBadge value={item.method} />,
  };
}

function PointDetail({ detail, timezone }: { detail: ValuationPointDetail; timezone?: string }) {
  const { portfolioId = "" } = useParams();
  const [openAccount, setOpenAccount] = useState<string | undefined>();
  const platforms = usePlatforms();
  const point = detail.point;
  return (
    <>
      <dl className={styles.metaList}>
        <div>
          <dt>Boundary flows</dt>
          <dd>
            <NumericText>{formatBoundaryFlows(point)}</NumericText>
          </dd>
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
        onRowClick={(key) => setOpenAccount((current) => (current === key ? undefined : key))}
        openKey={openAccount}
        detailId={`historical-positions-${point.id}`}
        renderDetail={(row) => (
          <HistoricalPositions
            portfolioId={portfolioId}
            accountId={typeof row.id === "string" ? row.id : ""}
            pointId={point.id}
          />
        )}
        columns={ACCOUNT_VALUATION_COLUMNS}
        rows={detail.accounts.map((item) => ({
          ...accountValuationRow(item, timezone, platforms.data?.items),
          account: (
            <button
              type="button"
              className={styles.inlineButton}
              aria-expanded={openAccount === item.accountId}
              aria-controls={`historical-positions-${point.id}`}
              onClick={() =>
                setOpenAccount((current) =>
                  current === item.accountId ? undefined : item.accountId,
                )
              }
            >
              {item.identityName} / {item.accountName}
            </button>
          ),
        }))}
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
