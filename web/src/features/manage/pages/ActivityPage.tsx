import { useParams } from "react-router";
import { activityTabFromSearch } from "@/features/manage/lib/tabs";
import { AuditPanel } from "@/features/manage/activity/AuditPanel";
import { CyclesPanel } from "@/features/manage/activity/CyclesPanel";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import shared from "@/shared/styles/page.module.css";

export function ActivityPage() {
  const { portfolioId = "" } = useParams();
  const { params, update } = useSearchState();
  const tab = activityTabFromSearch(params.toString());
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.timezone;

  return (
    <section className={shared.wide}>
      {tab === "cycles" ? (
        <CyclesPanel
          selectedId={params.get("cycle") ?? ""}
          timezone={timezone}
          onSelect={(cycleId) => update({ cycle: cycleId })}
        />
      ) : null}
      {tab === "audit" ? (
        <AuditPanel
          action={params.get("action") ?? ""}
          resourceType={params.get("resourceType") ?? ""}
          timezone={timezone}
          onFilter={update}
        />
      ) : null}
    </section>
  );
}
