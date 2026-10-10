import { useEffect } from "react";
import { useParams } from "react-router";
import { catalogTabFromSearch } from "@/features/manage/lib/tabs";
import { FuturesTab } from "../components/FuturesTab";
import { AssetsTab } from "../components/AssetsTab";
import { useCatalogFilters } from "../hooks/useCatalogFilters";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import shared from "@/shared/styles/page.module.css";

export function CatalogPage() {
  const { portfolioId = "" } = useParams();
  const { params, update } = useSearchState();
  const tab = catalogTabFromSearch(params.toString());
  useEffect(() => {
    const legacyTab = params.get("tab");
    if (legacyTab === "listings" || legacyTab === "instruments") {
      update({ tab, offset: params.get("offset") ?? "" });
    }
  }, [params, tab, update]);
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.timezone;
  const filters = useCatalogFilters();

  return (
    <section className={shared.wide}>
      {tab === "assets" ? (
        <AssetsTab portfolioId={portfolioId} filters={filters} timezone={timezone} />
      ) : (
        <FuturesTab portfolioId={portfolioId} filters={filters} timezone={timezone} />
      )}
    </section>
  );
}
