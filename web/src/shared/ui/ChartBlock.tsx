import type { ReactNode } from "react";
import { useChart } from "@/shared/lib/useChart";
import type { EChartsCoreOption } from "echarts/core";
import { DataTable, type DataTableProps } from "@/shared/ui/DataTable";
import styles from "@/shared/styles/page.module.css";

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
  const hostRef = useChart(option);

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
