import { useLocation } from "react-router";
import { pageTitle } from "@/app/shell/navigation";
import styles from "@/features/pages.module.css";

const phaseCopy: Record<string, string> = {
  overview: "Phase 2 adds valuation, capital, and issue summaries on this page.",
  balances: "Phase 2 adds live Balance tables from MySQL-backed projections.",
  positions: "Phase 2 adds live Position tables from MySQL-backed projections.",
  exposure: "Phase 3 adds long, short, and net Exposure views.",
  valuations: "Phase 3 adds valuation history charts and accessible tables.",
  "value-changes": "Phase 3 adds Value Change history for calendar periods.",
  baseline: "Phase 2 adds Capital Baseline editing.",
  flows: "Phase 2 adds Capital Flow revisions.",
  identities: "Phase 3 adds Identity management UI. The JSON API already exists.",
  connections: "Phase 3 adds Connection credential management. The JSON API already exists.",
  catalog: "Phase 3 adds catalog mapping and tracking screens.",
  observations: "Phase 3 adds Observation inspection and correction.",
  jobs: "Phase 3 adds a Jobs UI. Rebuilds are already enqueued from the CLI.",
};

export function PlaceholderPage() {
  const location = useLocation();
  const title = pageTitle(location.pathname);
  const key = location.pathname.split("/").at(-1) ?? "";
  return (
    <section className={styles.page}>
      <h2>{title}</h2>
      <p className={styles.notice}>
        {phaseCopy[key] ?? "This route is reserved for a later implementation phase."}
      </p>
    </section>
  );
}
