import { NumericText } from "@/shared/ui/NumericText";
import { useQuery } from "@tanstack/react-query";
import type { EChartsCoreOption } from "echarts/core";
import { type FormEvent, useMemo, useState } from "react";
import { getCustomValueChange, listValueChanges, getHistoryWindow } from "@/api/generated/sdk.gen";
import type { ValueChange } from "@/api/generated/types.gen";
import {
  BOUNDARY_COLUMNS,
  PERIOD_KINDS,
  boundaryEventRow,
} from "@/features/history/ValuationPointsPage";
import { chartNumber, formatAmount, formatUsd, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { readChoice } from "@/shared/lib/searchParams";
import { unwrapOrRedirect } from "@/shared/lib/session";
import { capitalize, reasonText } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { HistoryFilters, HistoryPager } from "./HistoryFilters";
import { useHistoryWindow, useHistoryPager } from "./useHistoryWindow";
import styles from "@/features/pages.module.css";

interface CustomRange {
  from: string;
  to: string;
}

export function ValueChangesPage() {
  const history = useHistoryWindow();
  const { portfolioId, userId, redirect, params, update, window } = history;
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, history.defaultPeriod);
  const from = window?.from;
  const to = window?.to;
  const [customFrom, setCustomFrom] = useState("");
  const [customTo, setCustomTo] = useState("");
  const [customError, setCustomError] = useState<string | null>(null);
  const [range, setRange] = useState<CustomRange | null>(null);

  const changes = useQuery({
    queryKey: ["value-changes", userId, portfolioId, periodKind, from, to],
    enabled: Boolean(userId && portfolioId && window),
    queryFn: async () =>
      unwrapOrRedirect(
        await listValueChanges({
          path: { portfolioId },
          query: { periodKind, from, to },
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
  const timezone = window?.context.timezone;
  const changeItems = [...(data?.items ?? [])].reverse();
  const pager = useHistoryPager("changes", changeItems.length);

  const chartOption = useMemo((): EChartsCoreOption | null => {
    const items = data?.items ?? [];
    // Periods without a value at both ends draw nothing, so an all-blank chart is no chart.
    if (items.every((item) => chartNumber(item.amountUsd) === null)) {
      return null;
    }
    return {
      grid: {
        left: 8,
        right: 12,
        top: 20,
        bottom: 8,
        containLabel: true,
      },
      xAxis: {
        type: "category",
        data: items.map((item) => item.label),
        axisLabel: { hideOverlap: true, fontSize: 11 },
      },
      yAxis: {
        type: "value",
        splitLine: { lineStyle: { opacity: 0.15 } },
      },
      series: [
        {
          type: "bar",
          name: "Value change",
          data: items.map((item) => {
            const val = chartNumber(item.amountUsd);
            if (val === null) return null;
            return {
              value: val,
              itemStyle: {
                borderRadius: val >= 0 ? [4, 4, 0, 0] : [0, 0, 4, 4],
              },
            };
          }),
          tooltip: {
            valueFormatter: (_value: unknown, index: number) => {
              const item = items[index];
              if (!item) return "—";
              const dec = parseDecimal(item.amountUsd);
              if (!dec) return "—";
              const sign = dec.gt(0) ? "+" : dec.lt(0) ? "-" : "";
              return `${sign}$${formatAmount(dec.abs().toFixed())} USD`;
            },
          },
        },
      ],
    };
  }, [data]);

  async function submitCustom(event: FormEvent) {
    event.preventDefault();
    setCustomError(null);
    try {
      const next = unwrapOrRedirect(
        await getHistoryWindow({
          path: { portfolioId },
          query: { fromLocal: customFrom, toLocal: customTo },
        }),
        redirect,
      );
      setRange(next);
    } catch (cause) {
      setCustomError(cause instanceof Error ? cause.message : "Enter a valid date and time.");
    }
  }

  return (
    <section className={styles.wide}>
      <HistoryFilters state={history} />
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
            action={
              <label className={styles.periodSwitcher}>
                <span className={styles.periodLabel}>Period</span>
                <Select
                  value={periodKind}
                  onChange={(event) =>
                    update({ periodKind: event.target.value, changesOffset: "" })
                  }
                  className={styles.periodSelect}
                >
                  {PERIOD_KINDS.map((option) => (
                    <option key={option} value={option}>
                      {capitalize(option)}
                    </option>
                  ))}
                </Select>
              </label>
            }
            option={chartOption}
            empty="No comparable period yet. A bar appears once a period has a Valuation Point at both ends."
            emptyRows="No periods yet. Periods appear once the Portfolio has Valuation Points."
            columns={VALUE_CHANGE_COLUMNS}
            rows={changeItems
              .slice(pager.offset, pager.offset + pager.limit)
              .map((item) => valueChangeRow(item, timezone))}
          />
          <HistoryPager
            label="Value Changes"
            pager={pager}
            total={changeItems.length}
            busy={changes.isFetching}
          />
        </>
      ) : null}
      <section className={styles.customCalcPanel}>
        <div className={styles.customCalcHeader}>
          <div>
            <h2 className={styles.customCalcTitle}>Custom Period Comparison</h2>
            <p className={styles.customCalcDesc}>
              Compare arbitrary valuation points and isolate capital flows and boundary events
              between them. Dates use {timezone ?? "the Portfolio timezone"}.
            </p>
          </div>
        </div>
        <form
          className={styles.rangeForm}
          onSubmit={(event) => {
            void submitCustom(event);
          }}
        >
          <label className={styles.rangeField}>
            <span className={styles.rangeLabel}>From</span>
            <TextInput
              type="datetime-local"
              step="1"
              value={customFrom}
              onChange={(event) => setCustomFrom(event.target.value)}
              required
            />
          </label>
          <label className={styles.rangeField}>
            <span className={styles.rangeLabel}>To</span>
            <TextInput
              type="datetime-local"
              step="1"
              value={customTo}
              onChange={(event) => setCustomTo(event.target.value)}
              required
            />
          </label>
          <div className={styles.rangeActions}>
            <ActionButton type="submit" tone="primary">
              Compute
            </ActionButton>
          </div>
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
        {range && custom.isPending ? <p className={styles.muted}>Computing comparison.</p> : null}
        {custom.data ? <CustomResult change={custom.data} timezone={timezone} /> : null}
      </section>
    </section>
  );
}

const VALUE_CHANGE_COLUMNS = [
  { key: "label", header: "Period" },
  { key: "amountUsd", header: "Value change (USD)", numeric: true },
  { key: "start", header: "Start", numeric: true },
  { key: "end", header: "End", numeric: true },
  { key: "capitalFlows", header: "Capital flows", numeric: true },
  { key: "boundaryFlows", header: "Boundary flows", numeric: true },
  { key: "completeness", header: "Completeness" },
  { key: "reasons", header: "Limitations" },
];

function valueChangeCell(amount: string | null | undefined) {
  const parsed = parseDecimal(amount);
  if (!parsed) {
    return <span className={styles.muted}>—</span>;
  }
  const isPositive = parsed.gt(0);
  const isNegative = parsed.lt(0);
  const sign = isPositive ? "+" : isNegative ? "-" : "";
  const className = isPositive
    ? styles.pnlPositive
    : isNegative
      ? styles.pnlNegative
      : styles.pnlNeutral;
  return (
    <span className={className}>
      <NumericText>{`${sign}$${formatAmount(parsed.abs().toFixed())}`}</NumericText>
    </span>
  );
}

function balanceCell(point: ValueChange["startPoint"]) {
  if (!point || !point.totalUsd) {
    return <span className={styles.muted}>—</span>;
  }
  return <NumericText>{formatUsd(point.totalUsd)}</NumericText>;
}

function flowsCell(inflow: string | null | undefined, outflow: string | null | undefined) {
  const inVal = parseDecimal(inflow);
  const outVal = parseDecimal(outflow);
  const hasIn = inVal && !inVal.isZero();
  const hasOut = outVal && !outVal.isZero();

  if (!hasIn && !hasOut) {
    return <span className={styles.muted}>—</span>;
  }
  if (hasIn && !hasOut) {
    return (
      <span className={styles.pnlPositive}>
        <NumericText>{`+$${formatAmount(inVal.toFixed())}`}</NumericText>
      </span>
    );
  }
  if (!hasIn && hasOut) {
    return (
      <span className={styles.pnlNegative}>
        <NumericText>{`-$${formatAmount(outVal.toFixed())}`}</NumericText>
      </span>
    );
  }
  return (
    <span className={styles.cellStack} style={{ alignItems: "flex-end" }}>
      <span className={styles.pnlPositive}>
        <NumericText>{`+$${formatAmount(inVal!.toFixed())}`}</NumericText>
      </span>
      <span className={styles.pnlNegative}>
        <NumericText>{`-$${formatAmount(outVal!.toFixed())}`}</NumericText>
      </span>
    </span>
  );
}

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
    amountUsd: valueChangeCell(item.amountUsd),
    start: balanceCell(item.startPoint),
    end: balanceCell(item.endPoint),
    capitalFlows: flowsCell(item.contributionsUsd, item.withdrawalsUsd),
    boundaryFlows: flowsCell(item.boundaryContributionsUsd, item.boundaryWithdrawalsUsd),
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
          <dd>{valueChangeCell(change.amountUsd)}</dd>
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
          <dd>
            <NumericText>{formatUsd(change.contributionsUsd)}</NumericText>
          </dd>
        </div>
        <div>
          <dt>Withdrawals</dt>
          <dd>
            <NumericText>{formatUsd(change.withdrawalsUsd)}</NumericText>
          </dd>
        </div>
        <div>
          <dt>Boundary in / out</dt>
          <dd>
            <NumericText>{formatUsd(change.boundaryContributionsUsd)}</NumericText> /{" "}
            <NumericText>{formatUsd(change.boundaryWithdrawalsUsd)}</NumericText>
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
        <p className={styles.notice}>Limitations: {change.reasons.map(reasonText).join(", ")}.</p>
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
      <NumericText>{formatUsd(point.totalUsd)}</NumericText> at{" "}
      <span className={styles.nowrap}>{formatInstant(point.evaluatedAt, timezone)}</span>
    </>
  );
}
