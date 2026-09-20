import type { EChartsCoreOption } from "echarts/core";
import { useMemo } from "react";
import { useParams } from "react-router";
import type { AmountStatus } from "@/api/generated/types.gen";
import { amountLabel, chartNumber, formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { IssueList } from "@/shared/ui/IssueList";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";

const MIN_TREND_POINTS = 4;

export function OverviewPage() {
  const { portfolioId = "" } = useParams();
  const query = useOverview(portfolioId);
  const overview = query.data;
  const unit = overview?.context.displayAsset.code;

  const valuationOption = useMemo((): EChartsCoreOption | null => {
    if (!overview) {
      return null;
    }
    const points = overview.valuationSeries.filter(
      (point) => point.completeness === "complete" && chartNumber(point.amount) !== null,
    );
    if (points.length < MIN_TREND_POINTS) {
      return null;
    }
    return {
      xAxis: {
        type: "category",
        data: points.map((point) => formatInstant(point.evaluatedAt, overview.context.timezone)),
      },
      yAxis: { type: "value", name: unit },
      series: [
        {
          type: "line",
          name: "Valuation",
          showSymbol: true,
          data: points.map((point) => chartNumber(point.amount)),
        },
      ],
    };
  }, [overview, unit]);

  const valueChangeOption = useMemo((): EChartsCoreOption | null => {
    if (!overview) {
      return null;
    }
    const months = overview.valueChanges.filter((point) => point.periodKind === "month");
    if (months.length === 0) {
      return null;
    }
    return {
      xAxis: { type: "category", data: months.map((point) => point.periodStartDate) },
      yAxis: { type: "value", name: unit },
      series: [
        {
          type: "bar",
          name: "Value change",
          data: months.map((point) => chartNumber(point.amount)),
          itemStyle: {
            borderType: "solid",
          },
        },
      ],
    };
  }, [overview, unit]);

  const exposureOption = useMemo((): EChartsCoreOption | null => {
    if (!overview || overview.exposures.length === 0) {
      return null;
    }
    const labels = overview.exposures.map((item) => item.assetCode ?? item.assetId);
    return {
      legend: { data: ["Long", "Short", "Net"] },
      xAxis: { type: "category", data: labels },
      yAxis: { type: "value", name: "Quantity" },
      series: [
        {
          type: "bar",
          name: "Long",
          data: overview.exposures.map((item) => chartNumber(item.longQuantity)),
        },
        {
          type: "bar",
          name: "Short",
          data: overview.exposures.map((item) => chartNumber(item.shortQuantity)),
          itemStyle: { borderWidth: 1, borderType: "dashed" },
        },
        {
          type: "line",
          name: "Net",
          data: overview.exposures.map((item) => chartNumber(item.netQuantity)),
          symbol: "diamond",
        },
      ],
    };
  }, [overview]);

  if (query.isPending) {
    return (
      <section className={styles.wide}>
        <h2>Overview</h2>
        <p className={styles.muted}>Loading projection overview.</p>
      </section>
    );
  }
  if (query.error || !overview) {
    return (
      <section className={styles.wide}>
        <h2>Overview</h2>
        <p className={styles.alert} role="alert">
          {query.error instanceof Error ? query.error.message : "Overview could not be loaded."}
        </p>
      </section>
    );
  }

  const completeValuationPoints = overview.valuationSeries.filter(
    (point) => point.completeness === "complete" && point.amount != null,
  ).length;

  return (
    <section className={styles.wide}>
      <h2>Overview</h2>
      <ContextSummary context={overview.context} />
      <div className={styles.cards}>
        <MetricCard title="Valuation" status={overview.valuation} unit={unit} />
        <MetricCard title="Contributed capital" status={overview.contributedCapital} unit={unit} />
        <MetricCard title="Value change" status={overview.valueChange} unit={unit} />
      </div>
      <ChartBlock
        title="Valuation"
        summary={
          completeValuationPoints < MIN_TREND_POINTS
            ? `Fewer than ${String(MIN_TREND_POINTS)} complete points, so no trend line is drawn. Exact strings remain in the table.`
            : `Line chart of ${String(completeValuationPoints)} complete valuation points in ${unit ?? "the display asset"}.`
        }
        option={valuationOption}
        empty={`Fewer than ${String(MIN_TREND_POINTS)} complete valuation points.`}
        columns={[
          { key: "evaluatedAt", header: "Evaluated at" },
          { key: "amount", header: "Amount", numeric: true },
          { key: "completeness", header: "Completeness" },
        ]}
        rows={overview.valuationSeries.map((point) => ({
          evaluatedAt: formatInstant(point.evaluatedAt, overview.context.timezone),
          amount: formatDecimal(point.amount),
          completeness: point.completeness,
        }))}
      />
      <ChartBlock
        title="Monthly value change"
        summary="Bars use server-calculated Value Change. Incomplete months stay blank instead of zero."
        option={valueChangeOption}
        empty="No monthly Value Change rows are available yet."
        columns={[
          { key: "period", header: "Period" },
          { key: "amount", header: "Amount", numeric: true },
          { key: "state", header: "State" },
        ]}
        rows={overview.valueChanges
          .filter((point) => point.periodKind === "month")
          .map((point) => ({
            period: `${point.periodStartDate} – ${point.periodEndDate}`,
            amount: formatDecimal(point.amount),
            state: point.state,
          }))}
      />
      <ChartBlock
        title="Exposure"
        summary="Long and short quantities stay visible even when net exposure is zero. Values are server quantities, not recomputed totals."
        option={exposureOption}
        empty="No exposure rows are available yet."
        columns={[
          { key: "asset", header: "Asset" },
          { key: "long", header: "Long", numeric: true },
          { key: "short", header: "Short", numeric: true },
          { key: "net", header: "Net", numeric: true },
          { key: "completeness", header: "Completeness" },
        ]}
        rows={overview.exposures.map((item) => ({
          asset: item.assetCode ?? item.assetId,
          long: formatDecimal(item.longQuantity),
          short: formatDecimal(item.shortQuantity),
          net: formatDecimal(item.netQuantity),
          completeness: item.completeness,
        }))}
      />
      <section className={styles.panel}>
        <h3>Issues</h3>
        <IssueList issues={overview.issues} />
      </section>
    </section>
  );
}

function MetricCard({
  title,
  status,
  unit,
}: {
  title: string;
  status: AmountStatus;
  unit?: string;
}) {
  return (
    <article className={styles.metric}>
      <h3>{title}</h3>
      <p className={styles.metricValue}>{amountLabel(status, unit)}</p>
      <p className={styles.muted}>
        {status.completeness} · {status.coverage} · {status.freshness}
        {status.state ? ` · ${status.state}` : ""}
      </p>
    </article>
  );
}
