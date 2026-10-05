import { useEffect, useRef, useState, type ReactNode } from "react";
import { BarChart, LineChart } from "echarts/charts";
import { GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { withChartTheme } from "@/shared/lib/chartTheme";
import { DataTable, type DataTableProps } from "@/shared/ui/DataTable";
import styles from "@/shared/styles/page.module.css";

echarts.use([
  BarChart,
  LineChart,
  GridComponent,
  LegendComponent,
  TooltipComponent,
  CanvasRenderer,
]);

interface ChartBlockProps extends Partial<Omit<DataTableProps, "caption" | "empty">> {
  title: string;
  summary?: string;
  action?: ReactNode;
  option: EChartsCoreOption | null;
  /** Shown instead of the chart when there is nothing to draw. */
  empty: string;
  /** Shown in the table when there are no rows; defaults to `empty`. */
  emptyRows?: string;
}

export function ChartBlock({
  title,
  summary,
  action,
  option,
  empty,
  emptyRows,
  ...table
}: ChartBlockProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [themeTick, setThemeTick] = useState(0);

  useEffect(() => {
    const root = document.documentElement;
    const observer = new MutationObserver(() => setThemeTick((value) => value + 1));
    observer.observe(root, {
      attributes: true,
      attributeFilter: ["data-resolved-theme", "data-theme"],
    });
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const host = hostRef.current;
    if (!host || !option) {
      return;
    }
    const chart = echarts.init(host);
    chart.setOption(withChartTheme(option), true);
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(host);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [option, themeTick]);

  return (
    <section className={styles.panel}>
      {action ? (
        <div className={styles.headingRow}>
          <h2>{title}</h2>
          {action}
        </div>
      ) : (
        <h2>{title}</h2>
      )}
      {summary ? <p className={styles.muted}>{summary}</p> : null}
      {option ? (
        <div ref={hostRef} className={styles.chart} role="img" aria-label={title} />
      ) : (
        <p className={styles.notice}>{empty}</p>
      )}
      {table.columns && table.rows && (table.rows.length > 0 || option) ? (
        <DataTable
          caption={`${title} values`}
          empty={emptyRows ?? empty}
          {...table}
          columns={table.columns}
          rows={table.rows}
        />
      ) : null}
    </section>
  );
}
