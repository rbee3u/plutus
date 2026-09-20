import { useEffect, useRef, useState } from "react";
import { BarChart, LineChart } from "echarts/charts";
import { GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { withChartTheme } from "@/shared/lib/chartTheme";
import { DataTable, type DataColumn } from "@/shared/ui/DataTable";
import styles from "@/features/pages.module.css";

echarts.use([
  BarChart,
  LineChart,
  GridComponent,
  LegendComponent,
  TooltipComponent,
  CanvasRenderer,
]);

interface ChartBlockProps {
  title: string;
  summary: string;
  option: EChartsCoreOption | null;
  columns: DataColumn[];
  rows: Record<string, string>[];
  empty: string;
}

export function ChartBlock({ title, summary, option, columns, rows, empty }: ChartBlockProps) {
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
    const resize = () => chart.resize();
    window.addEventListener("resize", resize);
    return () => {
      window.removeEventListener("resize", resize);
      chart.dispose();
    };
  }, [option, themeTick]);

  return (
    <section className={styles.panel}>
      <h3>{title}</h3>
      <p className={styles.muted}>{summary}</p>
      {option ? (
        <div ref={hostRef} className={styles.chart} role="img" aria-label={title} />
      ) : (
        <p className={styles.notice}>{empty}</p>
      )}
      <DataTable caption={`${title} exact values`} columns={columns} rows={rows} />
    </section>
  );
}
