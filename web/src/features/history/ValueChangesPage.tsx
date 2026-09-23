import { useQuery } from "@tanstack/react-query";
import type { EChartsCoreOption } from "echarts/core";
import { type FormEvent, useMemo, useState } from "react";
import { useParams } from "react-router";
import { getCustomValueChange, listValueChanges } from "@/api/generated/sdk.gen";
import type { ValueChange } from "@/api/generated/types.gen";
import {
  BOUNDARY_COLUMNS,
  PERIOD_KINDS,
  boundaryEventRow,
} from "@/features/history/ValuationPointsPage";
import { chartNumber, formatAmount, formatUsd } from "@/shared/lib/amount";
import { formatInstant, fromDatetimeLocalValue, isoFromLocalInput } from "@/shared/lib/datetime";
import { readChoice, useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { capitalize, reasonText } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

interface CustomRange {
  from: string;
  to: string;
}

export function ValueChangesPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const { params, update } = useSearchState();
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, "month");
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const [customFrom, setCustomFrom] = useState("");
  const [customTo, setCustomTo] = useState("");
  const [customError, setCustomError] = useState<string | null>(null);
  const [range, setRange] = useState<CustomRange | null>(null);

  const changes = useQuery({
    queryKey: ["value-changes", userId, portfolioId, periodKind, from, to],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listValueChanges({
          path: { portfolioId },
          query: { periodKind, from: isoFromLocalInput(from), to: isoFromLocalInput(to) },
        }),
        redirect,
      ),
  });
  const custom = useQuery({
    queryKey: ["custom-value-change", userId, portfolioId, range?.from ?? "", range?.to ?? ""],
    enabled: Boolean(userId && portfolioId && range),
    queryFn: async () =>
      unwrapOrRedirect(
        await getCustomValueChange({
          path: { portfolioId },
          query: { from: range?.from ?? "", to: range?.to ?? "" },
        }),
        redirect,
      ),
  });
  const data = changes.data;
  const timezone = data?.context.timezone;

  const chartOption = useMemo((): EChartsCoreOption | null => {
    const items = data?.items ?? [];
    // Periods without a value at both ends draw nothing, so an all-blank chart is no chart.
    if (items.every((item) => chartNumber(item.amountUsd) === null)) {
      return null;
    }
    return {
      xAxis: { type: "category", data: items.map((item) => item.label) },
      yAxis: { type: "value" },
      series: [
        {
          type: "bar",
          name: "Value change",
          data: items.map((item) => chartNumber(item.amountUsd)),
        },
      ],
    };
  }, [data]);

  function submitCustom(event: FormEvent) {
    event.preventDefault();
    setCustomError(null);
    try {
      const next = {
        from: fromDatetimeLocalValue(customFrom),
        to: fromDatetimeLocalValue(customTo),
      };
      if (next.from >= next.to) {
        setCustomError("The end must be after the start.");
        return;
      }
      setRange(next);
    } catch (cause) {
      setCustomError(cause instanceof Error ? cause.message : "Enter a valid date and time.");
    }
  }

  return (
    <section className={styles.wide}>
      <p className={styles.lead}>
        Value Change = end − start − contributions + withdrawals, with Boundary Events as implicit
        Capital Flows. A period missing a point stays blank, never zero.
      </p>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
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
      </form>
      <QueryRecovery
        error={changes.error}
        hasData={Boolean(data)}
        busy={changes.isFetching}
        fallback="Value Changes could not be loaded."
        onRetry={() => {
          void changes.refetch();
        }}
      />
      {changes.isPending && !data ? <p className={styles.muted}>Loading Value Changes.</p> : null}
      {data ? (
        <>
          <ChartBlock
            title={`Value change per ${data.periodKind}`}
            summary={`Periods close in ${data.context.timezone}. Incomplete or incomparable periods stay blank.`}
            option={chartOption}
            empty="No comparable period yet. A bar appears once a period has a Valuation Point at both ends."
            emptyRows="No periods yet. Periods appear once the Portfolio has Valuation Points."
            columns={VALUE_CHANGE_COLUMNS}
            rows={data.items.map((item) => valueChangeRow(item, timezone))}
          />
        </>
      ) : null}
      <section className={styles.panel}>
        <h2>Custom range</h2>
        <p className={styles.muted}>
          Compares the last Valuation Point at or before each instant, with the Capital Flows and
          Boundary Events between them.
        </p>
        <form className={styles.rangeForm} onSubmit={submitCustom}>
          <label className={styles.field}>
            <span>From</span>
            <TextInput
              type="datetime-local"
              value={customFrom}
              onChange={(event) => setCustomFrom(event.target.value)}
              required
            />
          </label>
          <label className={styles.field}>
            <span>To</span>
            <TextInput
              type="datetime-local"
              value={customTo}
              onChange={(event) => setCustomTo(event.target.value)}
              required
            />
          </label>
          <ActionButton type="submit" tone="primary">
            Compute
          </ActionButton>
        </form>
        {customError ? (
          <p className={styles.alert} role="alert">
            {customError}
          </p>
        ) : null}
        <QueryRecovery
          error={custom.error}
          hasData={Boolean(custom.data)}
          busy={custom.isFetching}
          fallback="The custom Value Change could not be computed."
          onRetry={() => {
            void custom.refetch();
          }}
        />
        {range && custom.isPending ? <p className={styles.muted}>Computing.</p> : null}
        {custom.data ? <CustomResult change={custom.data} timezone={timezone} /> : null}
      </section>
    </section>
  );
}

const VALUE_CHANGE_COLUMNS = [
  { key: "label", header: "Period" },
  { key: "amountUsd", header: "Value change (USD)", numeric: true },
  { key: "start", header: "Start (USD)", numeric: true },
  { key: "end", header: "End (USD)", numeric: true },
  { key: "contributionsUsd", header: "Contributions", numeric: true },
  { key: "withdrawalsUsd", header: "Withdrawals", numeric: true },
  { key: "boundaryContributionsUsd", header: "Boundary in", numeric: true },
  { key: "boundaryWithdrawalsUsd", header: "Boundary out", numeric: true },
  { key: "completeness", header: "Completeness" },
  { key: "reasons", header: "Missing" },
];

function valueChangeRow(item: ValueChange, timezone?: string): DataRow {
  return {
    id: `${item.periodKind}-${item.from}`,
    label: (
      <span className={styles.cellStack}>
        <span>{item.label}</span>
        <span className={`${styles.cellMeta} ${styles.nowrap}`}>
          {formatInstant(item.from, timezone)} – {formatInstant(item.to, timezone)}
        </span>
      </span>
    ),
    amountUsd: formatAmount(item.amountUsd),
    // A missing point is an unknown value; the Reasons column names which end is missing.
    start: item.startPoint ? formatAmount(item.startPoint.totalUsd) : "—",
    end: item.endPoint ? formatAmount(item.endPoint.totalUsd) : "—",
    contributionsUsd: formatAmount(item.contributionsUsd),
    withdrawalsUsd: formatAmount(item.withdrawalsUsd),
    boundaryContributionsUsd: formatAmount(item.boundaryContributionsUsd),
    boundaryWithdrawalsUsd: formatAmount(item.boundaryWithdrawalsUsd),
    completeness: <CompletenessBadge value={item.completeness} />,
    reasons: item.reasons.length > 0 ? item.reasons.map(reasonText).join(", ") : "—",
  };
}

function CustomResult({ change, timezone }: { change: ValueChange; timezone?: string }) {
  return (
    <div className={styles.detailPanel}>
      <dl className={styles.metaList}>
        <div>
          <dt>Range</dt>
          <dd>
            <span className={styles.nowrap}>{formatInstant(change.from, timezone)}</span> –{" "}
            <span className={styles.nowrap}>{formatInstant(change.to, timezone)}</span>
          </dd>
        </div>
        <div>
          <dt>Value change</dt>
          <dd>{formatUsd(change.amountUsd)}</dd>
        </div>
        <div>
          <dt>Start point</dt>
          <dd>
            <PointAt point={change.startPoint} timezone={timezone} />
          </dd>
        </div>
        <div>
          <dt>End point</dt>
          <dd>
            <PointAt point={change.endPoint} timezone={timezone} />
          </dd>
        </div>
        <div>
          <dt>Contributions</dt>
          <dd>{formatUsd(change.contributionsUsd)}</dd>
        </div>
        <div>
          <dt>Withdrawals</dt>
          <dd>{formatUsd(change.withdrawalsUsd)}</dd>
        </div>
        <div>
          <dt>Boundary in / out</dt>
          <dd>
            {formatUsd(change.boundaryContributionsUsd)} /{" "}
            {formatUsd(change.boundaryWithdrawalsUsd)}
          </dd>
        </div>
        <div>
          <dt>Completeness</dt>
          <dd>
            <CompletenessBadge value={change.completeness} />
          </dd>
        </div>
      </dl>
      {change.reasons.length > 0 ? (
        <p className={styles.notice}>Missing: {change.reasons.map(reasonText).join(", ")}.</p>
      ) : null}
      <DataTable
        caption="Boundary Events in the range"
        columns={BOUNDARY_COLUMNS}
        rows={change.boundaryEvents.map((item) => boundaryEventRow(item, timezone))}
        empty="No account entered or left the boundary in this range."
      />
    </div>
  );
}

/** A point's total and instant; a missing point is an unknown value, and the timestamp never breaks. */
function PointAt({ point, timezone }: { point: ValueChange["startPoint"]; timezone?: string }) {
  if (!point) {
    return "—";
  }
  return (
    <>
      {formatUsd(point.totalUsd)} at{" "}
      <span className={styles.nowrap}>{formatInstant(point.evaluatedAt, timezone)}</span>
    </>
  );
}
