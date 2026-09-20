import { listValuations } from "@/api/generated/sdk.gen";
import type { ValuationList } from "@/api/generated/types.gen";
import { chartNumber, formatDecimal } from "@/shared/lib/amount";
import { formatInstant, isoFromLocalInput } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { IssueList } from "@/shared/ui/IssueList";
import { CursorPager } from "@/shared/ui/Pager";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";
import type { EChartsCoreOption } from "echarts/core";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

const PAGE_SIZE = 50;
const GRAINS = ["", "auto", "hour", "day", "iso_week", "month", "quarter", "year"] as const;

export function ValuationsPage() {
  const { portfolioId = "" } = useParams();
  const overview = useOverview(portfolioId);
  const [params, setParams] = useSearchParams();
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
      <h2>Valuations</h2>
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
            {GRAINS.map((option) => (
              <option key={option || "default"} value={option}>
                {option || "Auto"}
              </option>
            ))}
          </Select>
        </label>
      </form>
      <ValuationResults
        key={`${from}|${to}|${granularity}|${generationId}`}
        portfolioId={portfolioId}
        from={from}
        to={to}
        granularity={granularity}
        generationId={generationId}
      />
    </section>
  );
}

function ValuationResults({
  portfolioId,
  from,
  to,
  granularity,
  generationId,
}: {
  portfolioId: string;
  from: string;
  to: string;
  granularity: (typeof GRAINS)[number];
  generationId: string;
}) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const [cursors, setCursors] = useState<string[]>([""]);
  const cursor = cursors[cursors.length - 1] ?? "";
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const query = useQuery({
    queryKey: [
      "valuations",
      user.data?.id,
      portfolioId,
      generationId,
      from,
      to,
      granularity,
      cursor,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<ValuationList> =>
      unwrapOrRedirect(
        await listValuations({
          path: { portfolioId },
          query: {
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
  const unit = query.data?.context.displayAsset.code;
  const chartOption = useMemo((): EChartsCoreOption | null => {
    const items = (query.data?.items ?? []).filter(
      (point) => point.completeness === "complete" && chartNumber(point.amount) !== null,
    );
    if (items.length < 2) {
      return null;
    }
    return {
      xAxis: {
        type: "category",
        data: items.map((point) => formatInstant(point.evaluatedAt, query.data?.context.timezone)),
      },
      yAxis: { type: "value", name: unit },
      series: [
        {
          type: "line",
          name: "Valuation",
          showSymbol: true,
          data: items.map((point) => chartNumber(point.amount)),
        },
      ],
    };
  }, [query.data, unit]);

  return (
    <>
      {query.error ? (
        <p className={styles.alert} role="alert">
          {query.error instanceof Error ? query.error.message : "Valuations could not be loaded."}
        </p>
      ) : null}
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
        title="Portfolio valuation"
        summary="The chart uses complete points only. Incomplete or missing amounts stay blank instead of zero."
        option={chartOption}
        empty="Fewer than two complete valuation points in this window."
        columns={[
          { key: "evaluatedAt", header: "Evaluated at" },
          { key: "amount", header: "Amount", numeric: true },
          { key: "completeness", header: "Completeness" },
        ]}
        rows={(query.data?.items ?? []).map((point) => ({
          evaluatedAt: formatInstant(point.evaluatedAt, query.data?.context.timezone),
          amount: formatDecimal(point.amount),
          completeness: point.completeness,
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
      {query.data ? (
        <section className={styles.panel}>
          <h3>Issues</h3>
          <IssueList issues={query.data.issues} />
        </section>
      ) : null}
    </>
  );
}

function readGrain(value: string | null): (typeof GRAINS)[number] {
  return GRAINS.includes(value as (typeof GRAINS)[number])
    ? (value as (typeof GRAINS)[number])
    : "";
}
