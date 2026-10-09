import { useParams } from "react-router";
import { catalogTabFromSearch } from "@/features/manage/lib/tabs";
import { InstrumentsTab } from "../components/InstrumentsTab";
import { ListingsTab } from "../components/ListingsTab";
import { useCatalogFilters } from "../hooks/useCatalogFilters";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import shared from "@/shared/styles/page.module.css";

export function CatalogPage() {
  const { portfolioId = "" } = useParams();
  const { params } = useSearchState();
  const tab = catalogTabFromSearch(params.toString());
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.item.timezone;
  const filters = useCatalogFilters();

  return (
    <section className={shared.wide}>
      {tab === "listings" ? (
        <ListingsTab portfolioId={portfolioId} filters={filters} timezone={timezone} />
      ) : (
        <InstrumentsTab portfolioId={portfolioId} filters={filters} timezone={timezone} />
      )}
    </section>
  );
}
