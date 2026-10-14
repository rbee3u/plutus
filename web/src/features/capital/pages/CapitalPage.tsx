import { useParams } from "react-router";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import { capitalTabFromSearch } from "../navigation";
import { CapitalFlowsPanel } from "../components/CapitalFlowsPanel";
import { BoundaryEventsPanel } from "../components/BoundaryEventsPanel";
import shared from "@/shared/styles/page.module.css";

export function CapitalPage() {
  const { portfolioId = "" } = useParams();
  const { params } = useSearchState();
  const tab = capitalTabFromSearch(params.toString());
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.timezone;

  return (
    <section className={shared.wide}>
      {tab === "flows" ? (
        <CapitalFlowsPanel portfolioId={portfolioId} timezone={timezone} />
      ) : (
        <BoundaryEventsPanel timezone={timezone} />
      )}
    </section>
  );
}
