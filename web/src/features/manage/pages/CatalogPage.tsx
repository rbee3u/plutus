import { type FormEvent, useState } from "react";
import { useParams } from "react-router";
import { catalogTabFromSearch } from "@/features/manage/lib/tabs";
import { InstrumentsTab } from "../components/InstrumentsTab";
import { ListingsTab } from "../components/ListingsTab";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolio } from "@/shared/lib/portfolio";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Select, TextInput } from "@/shared/ui/FormControls";
import shared from "@/shared/styles/page.module.css";

export function CatalogPage() {
  const { portfolioId = "" } = useParams();
  const { params, update } = useSearchState();
  const tab = catalogTabFromSearch(params.toString());
  const platform = params.get("platform") ?? "";
  const q = params.get("q") ?? "";
  const ignoredOnly = params.get("ignoredOnly") === "true";
  const offset = readOffset(params);
  const platforms = usePlatforms();
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.item.timezone;
  const [draftQuery, setDraftQuery] = useState(q);

  function submitSearch(event: FormEvent) {
    event.preventDefault();
    update({ q: draftQuery.trim() });
  }

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
      <form
        className={shared.compactFilterBar}
        style={{ marginBottom: "var(--space-md)" }}
        onSubmit={submitSearch}
      >
        <label className={shared.compactFilterField}>
          <span className={shared.compactFilterLabel}>Platform</span>
          <Select
            value={platform}
            onChange={(event) => update({ platform: event.target.value, offset: "" })}
          >
            <option value="">All platforms</option>
            {(platforms.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </label>
        {tab === "listings" ? (
          <label className={shared.compactFilterField}>
            <span className={shared.compactFilterLabel}>Treatment</span>
            <Select
              value={ignoredOnly ? "ignored" : "all"}
              onChange={(event) =>
                update({
                  ignoredOnly: event.target.value === "ignored" ? "true" : "",
                  offset: "",
                })
              }
            >
              <option value="all">All listings</option>
              <option value="ignored">Ignored listings</option>
            </Select>
          </label>
        ) : null}
        <label
          className={shared.compactFilterField}
          style={{ flex: "1 1 200px", maxWidth: "320px" }}
        >
          <span className={shared.compactFilterLabel}>Search</span>
          <TextInput
            value={draftQuery}
            onChange={(event) => setDraftQuery(event.target.value)}
            placeholder="Symbol or name"
          />
        </label>
        <ActionButton type="submit">Search</ActionButton>
        {q ? (
          <ActionButton
            type="button"
            tone="ghost"
            onClick={() => {
              setDraftQuery("");
              update({ q: "", offset: "" });
            }}
          >
            Clear
          </ActionButton>
        ) : null}
      </form>
      {tab === "listings" ? (
        <ListingsTab
          portfolioId={portfolioId}
          platform={platform}
          q={q}
          ignoredOnly={ignoredOnly}
          offset={offset}
          timezone={timezone}
          onOffset={(next) => update({ offset: next > 0 ? String(next) : "" })}
        />
      ) : (
        <InstrumentsTab
          portfolioId={portfolioId}
          platform={platform}
          q={q}
          offset={offset}
          timezone={timezone}
          onOffset={(next) => update({ offset: next > 0 ? String(next) : "" })}
        />
      )}
    </section>
  );
}
