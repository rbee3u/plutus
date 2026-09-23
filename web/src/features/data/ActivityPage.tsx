import { useParams } from "react-router";
import { activityTabFromSearch } from "@/app/shell/navigation";
import { AuditPanel } from "@/features/data/activity/AuditPanel";
import { CyclesPanel } from "@/features/data/activity/CyclesPanel";
import { JOB_KINDS, JobsPanel } from "@/features/data/activity/JobsPanel";
import { usePortfolio } from "@/shared/lib/portfolio";
import { readChoice, useSearchState } from "@/shared/lib/searchParams";
import styles from "@/features/pages.module.css";

export function ActivityPage() {
  const { portfolioId = "" } = useParams();
  const { params, update } = useSearchState();
  const tab = activityTabFromSearch(params.toString());
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.item.timezone;

  return (
    <section className={styles.wide}>
      {tab === "cycles" ? (
        <CyclesPanel
          portfolioId={portfolioId}
          selectedId={params.get("cycle") ?? ""}
          timezone={timezone}
          onSelect={(cycleId) => update({ cycle: cycleId })}
        />
      ) : null}
      {tab === "jobs" ? (
        <JobsPanel
          portfolioId={portfolioId}
          kind={readChoice(params.get("kind"), JOB_KINDS, "" as const)}
          timezone={timezone}
          onKind={(kind) => update({ kind })}
        />
      ) : null}
      {tab === "audit" ? (
        <AuditPanel
          portfolioId={portfolioId}
          action={params.get("action") ?? ""}
          resourceType={params.get("resourceType") ?? ""}
          timezone={timezone}
          onFilter={update}
        />
      ) : null}
    </section>
  );
}
