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
      <div className={shared.panelHeader} style={{ flexWrap: "wrap", gap: "var(--space-md)" }}>
        <div>
          <h2 style={{ margin: 0 }}>
            {tab === "listings" ? "Asset Listings" : "Instruments Catalog"}
          </h2>
          <p className={shared.muted} style={{ margin: "4px 0 0", fontSize: "var(--text-sm)" }}>
            {tab === "listings"
              ? "Raw asset listings and custom symbol mappings discovered from connected exchanges."
              : "Futures and derivatives contracts metadata discovered from connected exchanges."}
          </p>
        </div>
      </div>
      {tab === "listings" ? (
        <ListingsTab portfolioId={portfolioId} filters={filters} timezone={timezone} />
      ) : (
        <InstrumentsTab portfolioId={portfolioId} filters={filters} timezone={timezone} />
      )}
    </section>
  );
}
