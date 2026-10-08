import type { EChartsCoreOption } from "echarts/core";
import { ChevronRight } from "lucide-react";
import { useEffect, useMemo } from "react";
import {
  getValuationPoint,
  keys,
  listPeriodCloses,
  listValuationPointPage,
  listValuationPoints,
} from "@/api";
import type {
  PeriodCloseList,
  ValuationPointDetail,
  ValuationPointList,
  ValuationPointPage,
  ValuationPointSummary,
} from "@/api";
import { chartNumber, formatAmount, isZeroDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { readChoice } from "@/shared/lib/searchParams";
import { accountSummary, capitalize, reasonText } from "@/shared/lib/text";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Select } from "@/shared/ui/FormControls";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { HistoryFilters, HistoryPager } from "../components/HistoryFilters";
import { PointDetail, POINT_DETAIL_ID } from "../components/PointDetail";
import { useHistoryPager, useHistoryWindow } from "../hooks/useHistoryWindow";
import { historyGranularity, PERIOD_KINDS } from "@/features/history/lib/timeWindow";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

const NO_POINT =
  "No Valuation Point in this window. Enable a connection and run a Collection Cycle.";

export function ValuationPointsPage() {
  const history = useHistoryWindow();
  const { params, update, window } = history;
  const from = window?.from;
  const to = window?.to;
  const granularity = from && to ? historyGranularity(from, to) : "hour";
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, history.defaultPeriod);
  const pointId = params.get("point") ?? "";
  const pointPager = useHistoryPager("points");

  const points = usePortfolioQuery<ValuationPointList>({
    key: (userId, portfolioId) => keys.valuationPoints(userId, portfolioId, from, to, granularity),
    ready: Boolean(window),
    load: (portfolioId) =>
      listValuationPoints({ path: { portfolioId }, query: { from, to, granularity } }),
  });
  const records = usePortfolioQuery<ValuationPointPage>({
    key: (userId, portfolioId) =>
      keys.valuationPointPage(userId, portfolioId, from, to, pointPager.offset, pointPager.limit),
    ready: Boolean(window),
    load: (portfolioId) =>
      listValuationPointPage({
        path: { portfolioId },
        query: { from, to, offset: pointPager.offset, limit: pointPager.limit },
      }),
  });
  useEffect(() => {
    const total = records.data?.page.total;
    if (total !== undefined && pointPager.offset > 0 && pointPager.offset >= total) {
      pointPager.onChange(Math.max(0, Math.ceil(total / pointPager.limit) - 1) * pointPager.limit);
    }
  }, [records.data, pointPager]);
  const detail = usePortfolioQuery<ValuationPointDetail>({
    key: (userId, portfolioId) => keys.valuationPoint(userId, portfolioId, pointId),
    ready: Boolean(pointId),
    load: (portfolioId) => getValuationPoint({ path: { portfolioId, pointId } }),
  });
  const closes = usePortfolioQuery<PeriodCloseList>({
    key: (userId, portfolioId) => keys.periodCloses(userId, portfolioId, periodKind, from, to),
    ready: Boolean(window),
    load: (portfolioId) =>
      listPeriodCloses({ path: { portfolioId }, query: { periodKind, from, to } }),
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
    <section className={shared.wide}>
      <HistoryFilters state={history} />
      <QueryRecovery query={points} fallback="Valuation Points could not be loaded." />
      {points.isPending && !series ? (
        <p className={shared.muted}>Loading Valuation Points.</p>
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
      <section className={shared.panel}>
        <div className={shared.headingRow}>
          <h2>Period closes</h2>
          <label className={`${shared.field} ${styles.toolbarField}`}>
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
        <QueryRecovery query={closes} fallback="Period closes could not be loaded." />
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
                <span className={shared.cellStack}>
                  <span>{item.label}</span>
                  <span className={`${shared.cellMeta} ${shared.nowrap}`}>
                    {formatInstant(item.periodStart, timezone)} –{" "}
                    {formatInstant(item.periodEnd, timezone)}
                  </span>
                </span>
              ),
              totalUsd: item.point ? (
                formatAmount(item.point.totalUsd)
              ) : (
                <span className={shared.cellMeta}>no point</span>
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
      <section className={shared.panel} aria-labelledby="raw-points-heading">
        <h2 id="raw-points-heading">Raw Valuation Points</h2>
        <QueryRecovery query={records} fallback="Raw Valuation Points could not be loaded." />
        <DataTable
          caption="Raw Valuation Points"
          empty={records.isPending ? "Loading raw Valuation Points." : NO_POINT}
          rowClassName={(row) => (row.id === pointId ? shared.openRow : undefined)}
          openKey={pointId}
          onRowClick={(key) => update({ point: key === pointId ? "" : key })}
          detailId={POINT_DETAIL_ID}
          renderDetail={() => (
            <>
              <QueryRecovery query={detail} fallback="The Valuation Point could not be loaded." />
              {detail.data ? <PointDetail detail={detail.data} timezone={timezone} /> : null}
              {detail.isPending ? <p className={shared.muted}>Loading the point.</p> : null}
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
        <span className={shared.cellMeta}>{formatAmount(item.boundaryContributionUsd)}</span>
      ) : (
        `+${formatAmount(item.boundaryContributionUsd)} / −${formatAmount(item.boundaryWithdrawalUsd)}`
      ),
    actions: (
      <span className={shared.rowActions}>
        <IconButton
          label={`${open ? "Collapse" : "Expand"} the Valuation Point at ${evaluatedAt}`}
          aria-expanded={open}
          aria-controls={open ? POINT_DETAIL_ID : undefined}
          onClick={() => update({ point: open ? "" : item.id })}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? shared.chevronOpen : shared.chevron}
          />
        </IconButton>
      </span>
    ),
  };
}
