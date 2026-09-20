import { useLocation } from "react-router";
import { pageTitle } from "@/app/shell/navigation";
import styles from "@/features/pages.module.css";

const phaseCopy: Record<string, string> = {
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
