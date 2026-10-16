import type { EChartsCoreOption } from "echarts/core";
import { useMemo } from "react";
import { keys, listValueChanges } from "@/api";
import type { ValueChangeList } from "@/api";
import { chartNumber, formatAmount, parseDecimal } from "@/shared/lib/amount";
import { readChoice } from "@/shared/lib/searchParams";
import { capitalize } from "@/shared/lib/text";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { Select } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { ValueChangeSummary } from "../components/ValueChangeSummary";
import { HistoryFilters, HistoryPager } from "../components/HistoryFilters";
import { useHistoryPager, useHistoryWindow } from "../hooks/useHistoryWindow";
import { PERIOD_KINDS } from "@/features/history/lib/timeWindow";
import { VALUE_CHANGE_COLUMNS, valueChangeRow } from "../lib/valueChangeRows";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function ValueChangesPage() {
  const history = useHistoryWindow();
  const { params, update, window } = history;
  const periodKind = readChoice(params.get("periodKind"), PERIOD_KINDS, history.defaultPeriod);
  const from = window?.from;
  const to = window?.to;

  const changes = usePortfolioQuery<ValueChangeList>({
    key: (userId, portfolioId) => keys.valueChanges(userId, portfolioId, periodKind, from, to),
    ready: Boolean(window),
    load: (portfolioId) =>
      listValueChanges({ path: { portfolioId }, query: { periodKind, from, to } }),
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
              return `${sign}$${formatAmount(dec.abs().toFixed())}`;
            },
          },
        },
      ],
    };
  }, [data]);

  return (
    <section className={shared.wide}>
      <HistoryFilters state={history} />
      <ValueChangeSummary window={window} />
      <p className={shared.muted}>
        The summary compares valuation points at or before the selected endpoints. The chart and
        table show whole calendar periods, so their changes may not add up to the summary.
      </p>
      <QueryRecovery query={changes} fallback="Value changes could not be loaded." />
      {changes.isPending && !data ? <p className={shared.muted}>Loading value changes.</p> : null}
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
            empty="No comparable periods in this range. Adjust the range or run a collection cycle."
            emptyRows="No periods yet. Periods appear once the portfolio has valuation points."
            columns={VALUE_CHANGE_COLUMNS}
            rows={changeItems
              .slice(pager.offset, pager.offset + pager.limit)
              .map((item) => valueChangeRow(item, timezone))}
          />
          <HistoryPager
            label="Value changes"
            pager={pager}
            total={changeItems.length}
            busy={changes.isFetching}
          />
        </>
      ) : null}
    </section>
  );
}
