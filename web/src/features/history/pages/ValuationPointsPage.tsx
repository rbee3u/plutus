import { useValuationRevision } from "@/shared/lib/useValuationRevision";
import type { EChartsCoreOption } from "echarts/core";
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
} from "@/api";
import { chartNumber, formatAmount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { readChoice } from "@/shared/lib/searchParams";
import { capitalize } from "@/shared/lib/text";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable } from "@/shared/ui/DataTable";
import { Select } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { HistoryFilters, HistoryPager } from "../components/HistoryFilters";
import { PointDetail, POINT_DETAIL_ID } from "../components/PointDetail";
import { useHistoryPager, useHistoryWindow } from "../hooks/useHistoryWindow";
import { historyGranularity, PERIOD_KINDS } from "@/features/history/lib/timeWindow";
import {
  PERIOD_CLOSE_COLUMNS,
  RAW_POINT_COLUMNS,
  periodCloseRow,
  rawPointRow,
} from "../lib/valuationPointRows";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

const NO_POINT =
  "No valuation point in this window. Enable a connection and run a collection cycle.";

export function ValuationPointsPage() {
  const history = useHistoryWindow();
  const { params, update, window } = history;
  const from = window?.from;
  const to = window?.to;
  const granularity = from && to ? historyGranularity(from, to) : "hour";
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, history.defaultPeriod);
  const cycleId = params.get("point") ?? "";
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
  const selectedRevision = records.data?.items.find(
    (item) => item.cycleId === cycleId,
  )?.valuationRevision;
  const detail = usePortfolioQuery<ValuationPointDetail>({
    key: (userId, portfolioId) =>
      keys.valuationPoint(userId, portfolioId, cycleId, selectedRevision),
    ready: Boolean(cycleId && selectedRevision),
    load: (portfolioId) => getValuationPoint({ path: { portfolioId, cycleId } }),
  });
  const closes = usePortfolioQuery<PeriodCloseList>({
    key: (userId, portfolioId) => keys.periodCloses(userId, portfolioId, periodKind, from, to),
    ready: Boolean(window),
    load: (portfolioId) =>
      listPeriodCloses({ path: { portfolioId }, query: { periodKind, from, to } }),
  });
  const detailMatches = useValuationRevision(
    selectedRevision,
    detail.data?.point.valuationRevision,
  );
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
      <QueryRecovery query={points} fallback="Valuation points could not be loaded." />
      {points.isPending && !series ? (
        <p className={shared.muted}>Loading valuation points.</p>
      ) : null}
      {series ? (
        <ChartBlock
          title="Portfolio value"
          summary={`${formatInstant(series.from, timezone)} – ${formatInstant(series.to, timezone)} · ${series.granularity} granularity`}
          option={chartOption}
          empty={
            series.items.length === 0
              ? NO_POINT
              : "Only one valuation point in this window; the chart needs two."
          }
        />
      ) : null}
      <section className={shared.panel}>
        <div className={shared.headingRow}>
          <h2>Period closes</h2>
          <label className={styles.periodSwitcher}>
            <span className={styles.periodLabel}>Period</span>
            <Select
              value={periodKind}
              className={styles.periodSelect}
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
          columns={PERIOD_CLOSE_COLUMNS}
          rows={closeItems
            .slice(closePager.offset, closePager.offset + closePager.limit)
            .map((item, index) => {
              const globalIndex = closePager.offset + index;
              const prevItem = closeItems[globalIndex + 1];
              return periodCloseRow(item, prevItem, timezone);
            })}
          empty={
            closes.isPending
              ? "Loading period closes."
              : "No periods yet. Periods appear once the portfolio has a valuation point."
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
        <h2 id="raw-points-heading">Raw valuation points</h2>
        <QueryRecovery query={records} fallback="Raw valuation points could not be loaded." />
        <DataTable
          caption="Raw valuation points"
          empty={records.isPending ? "Loading raw valuation points." : NO_POINT}
          rowClassName={(row) => (row.id === cycleId ? shared.openRow : undefined)}
          openKey={cycleId}
          onRowClick={(key) => update({ point: key === cycleId ? "" : key })}
          detailId={POINT_DETAIL_ID}
          renderDetail={() => (
            <>
              <QueryRecovery query={detail} fallback="The valuation point could not be loaded." />
              {detailMatches && detail.data ? (
                <PointDetail detail={detail.data} timezone={timezone} />
              ) : null}
              {detail.isPending ? <p className={shared.muted}>Loading the point.</p> : null}
            </>
          )}
          columns={RAW_POINT_COLUMNS}
          rows={(records.data?.items ?? []).map((item, index, items) => {
            const prevItem = items[index + 1];
            return rawPointRow(item, prevItem, timezone, cycleId, update);
          })}
        />
        <HistoryPager
          label="Raw valuation points"
          pager={pointPager}
          total={records.data?.page.total ?? 0}
          busy={records.isFetching}
        />
      </section>
    </section>
  );
}
