import { listPeriodSummaries, listValueChanges } from "@/api/generated/sdk.gen";
import type {
  CalendarPeriodKind,
  PeriodSummaryList,
  TimeGranularity,
  ValueChangeList,
} from "@/api/generated/types.gen";
import { chartNumber, formatDecimal } from "@/shared/lib/amount";
import { formatDate, formatInstant, isoFromLocalInput } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { CursorPager } from "@/shared/ui/Pager";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";
import type { EChartsCoreOption } from "echarts/core";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

const PAGE_SIZE = 50;
const PERIODS = ["day", "iso_week", "month", "quarter", "year"] as const;
const GRAINS = ["", "auto", "hour", "day", "iso_week", "month", "quarter", "year"] as const;

export function ValueChangesPage() {
  const { portfolioId = "" } = useParams();
  const overview = useOverview(portfolioId);
  const [params, setParams] = useSearchParams();
  const periodKind = readPeriod(params.get("periodKind"));
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const granularity = readGrain(params.get("granularity"));
  const generationId = overview.data?.context.generationId ?? "";

  function update(next: Record<string, string>) {
    const merged = new URLSearchParams(params);
    for (const [key, value] of Object.entries(next)) {
      if (value) merged.set(key, value);
      else merged.delete(key);
    }
    setParams(merged, { replace: true });
  }

  return (
    <section className={styles.wide}>
      <h2>Value Changes</h2>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>Period</span>
          <Select
            value={periodKind}
            onChange={(event) => update({ periodKind: event.target.value })}
          >
            {PERIODS.map((option) => (
              <option key={option} value={option}>
                {option.replace("_", " ")}
              </option>
            ))}
          </Select>
        </label>
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
            {GRAINS.map((option) => (
              <option key={option || "default"} value={option}>
                {option || "Auto"}
              </option>
            ))}
          </Select>
        </label>
      </form>
      <ValueChangeResults
        key={`${periodKind}|${from}|${to}|${granularity}|${generationId}`}
        portfolioId={portfolioId}
        periodKind={periodKind}
        from={from}
        to={to}
        granularity={granularity}
        generationId={generationId}
      />
    </section>
  );
}

function ValueChangeResults({
  portfolioId,
  periodKind,
  from,
  to,
  granularity,
  generationId,
}: {
  portfolioId: string;
  periodKind: CalendarPeriodKind;
  from: string;
  to: string;
  granularity: TimeGranularity | "";
  generationId: string;
}) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const [cursors, setCursors] = useState<string[]>([""]);
  const [summaryCursors, setSummaryCursors] = useState<string[]>([""]);
  const cursor = cursors[cursors.length - 1] ?? "";
  const summaryCursor = summaryCursors[summaryCursors.length - 1] ?? "";
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const query = useQuery({
    queryKey: [
      "value-changes",
      user.data?.id,
      portfolioId,
      generationId,
      periodKind,
      from,
      to,
      granularity,
      cursor,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<ValueChangeList> =>
      unwrapOrRedirect(
        await listValueChanges({
          path: { portfolioId },
          query: {
            periodKind,
            from: isoFromLocalInput(from),
            to: isoFromLocalInput(to),
            granularity: granularity || undefined,
            limit: PAGE_SIZE,
            cursor: cursor || undefined,
          },
        }),
        redirect,
      ),
  });
  const summaries = useQuery({
    queryKey: [
      "period-summaries",
      user.data?.id,
      portfolioId,
      generationId,
      periodKind,
      from,
      to,
      summaryCursor,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<PeriodSummaryList> =>
      unwrapOrRedirect(
        await listPeriodSummaries({
          path: { portfolioId },
          query: {
            periodKind,
            from: isoFromLocalInput(from),
            to: isoFromLocalInput(to),
            limit: PAGE_SIZE,
            cursor: summaryCursor || undefined,
          },
        }),
        redirect,
      ),
  });
  const unit = query.data?.context.displayAsset.code;
  const chartOption = useMemo((): EChartsCoreOption | null => {
    const items = query.data?.items ?? [];
    if (items.length === 0) {
      return null;
    }
    return {
      xAxis: {
        type: "category",
        data: items.map((item) => formatDate(item.periodStartDate)),
      },
      yAxis: { type: "value", name: unit },
      series: [
        {
          type: "bar",
          name: "Value change",
          data: items.map((item) => chartNumber(item.amount)),
        },
      ],
    };
  }, [query.data, unit]);

  return (
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Value changes could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      {query.data ? <ContextSummary context={query.data.context} /> : null}
      {query.data?.granularity ? (
        <p className={styles.muted}>Sampled at {query.data.granularity} resolution.</p>
      ) : null}
      {query.data?.missingIntervals && query.data.missingIntervals.length > 0 ? (
        <p className={styles.notice}>
          Missing intervals:{" "}
          {query.data.missingIntervals
            .map(
              (item) =>
                `${formatInstant(item.start, query.data.context.timezone)} – ${formatInstant(item.end, query.data.context.timezone)}`,
            )
            .join("; ")}
        </p>
      ) : null}
      <ChartBlock
        title="Calendar value change"
        summary="Bars use server-calculated Value Change. Incomplete or incomparable periods stay blank instead of zero."
        option={chartOption}
        empty="No Value Change rows are available for this period kind."
        columns={[
          { key: "period", header: "Period" },
          { key: "amount", header: "Amount", numeric: true },
          { key: "contribution", header: "Contributions", numeric: true },
          { key: "withdrawal", header: "Withdrawals", numeric: true },
          { key: "state", header: "State" },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          period: `${formatDate(item.periodStartDate)} – ${formatDate(item.periodEndDate)}`,
          amount: formatDecimal(item.amount),
          contribution: formatDecimal(item.contributionAmount),
          withdrawal: formatDecimal(item.withdrawalAmount),
          state: item.state,
        }))}
      />
      <CursorPager
        canPrev={cursors.length > 1}
        hasMore={query.data?.page.hasMore ?? false}
        onPrev={() => setCursors((items) => items.slice(0, -1))}
        onNext={() => {
          const next = query.data?.page.nextCursor;
          if (next) setCursors((items) => [...items, next]);
        }}
      />
      <section className={styles.panel}>
        <h3>Period summaries</h3>
        <QueryRecovery
          error={summaries.error}
          hasData={Boolean(summaries.data)}
          busy={summaries.isFetching}
          fallback="Period summaries could not be loaded."
          onRetry={() => {
            void summaries.refetch();
          }}
        />
        <DataTable
          caption="Period open, close, and value-change summaries"
          columns={[
            { key: "period", header: "Period" },
            { key: "open", header: "Open", numeric: true },
            { key: "close", header: "Close", numeric: true },
            { key: "high", header: "High", numeric: true },
            { key: "low", header: "Low", numeric: true },
            { key: "capital", header: "Capital close", numeric: true },
            { key: "change", header: "Value change", numeric: true },
            { key: "completeness", header: "Completeness" },
          ]}
          rows={(summaries.data?.items ?? []).map((item) => ({
            period: `${formatDate(item.periodStartDate)} – ${formatDate(item.periodEndDate)}`,
            open: formatDecimal(item.valuationOpen),
            close: formatDecimal(item.valuationClose),
            high: formatDecimal(item.valuationHigh),
            low: formatDecimal(item.valuationLow),
            capital: formatDecimal(item.contributedCapitalClose),
            change: formatDecimal(item.valueChange),
            completeness: item.completeness,
          }))}
        />
        <CursorPager
          canPrev={summaryCursors.length > 1}
          hasMore={summaries.data?.page.hasMore ?? false}
          onPrev={() => setSummaryCursors((items) => items.slice(0, -1))}
          onNext={() => {
            const next = summaries.data?.page.nextCursor;
            if (next) setSummaryCursors((items) => [...items, next]);
          }}
        />
      </section>
      {query.data ? (
        <section className={styles.panel}>
          <h3>Issues</h3>
          <IssueList issues={query.data.issues} />
        </section>
      ) : null}
    </>
  );
}

function readPeriod(value: string | null): CalendarPeriodKind {
  return PERIODS.includes(value as (typeof PERIODS)[number])
    ? (value as CalendarPeriodKind)
    : "month";
}

function readGrain(value: string | null): TimeGranularity | "" {
  return GRAINS.includes(value as (typeof GRAINS)[number])
    ? (value as (typeof GRAINS)[number])
    : "";
}
