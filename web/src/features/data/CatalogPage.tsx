import { useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useRef, useState } from "react";
import { useParams } from "react-router";
import {
  listAssetListings,
  listInstruments,
  putAssetListingExclusion,
  putAssetListingMapping,
} from "@/api/generated/sdk.gen";
import type { AssetListing } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { catalogTabFromSearch } from "@/app/shell/navigation";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { usePortfolio } from "@/shared/lib/portfolio";
import { invalidatePortfolioReads } from "@/shared/lib/queryKeys";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { capitalize } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Badge, StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextArea, TextInput } from "@/shared/ui/FormControls";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export function listingEtag(item: AssetListing): string {
  return `"listing:${String(item.version)}"`;
}

function exclusionEtag(item: AssetListing): string {
  return `"listing-exclusion:${String(item.exclusionVersion)}"`;
}

interface MappingRequest {
  nonce: number;
  item: AssetListing;
}

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
    <section className={styles.wide}>
      <div className={styles.panelHeader} style={{ flexWrap: "wrap", gap: "var(--space-md)" }}>
        <div>
          <h2 style={{ margin: 0 }}>
            {tab === "listings" ? "Asset Listings" : "Instruments Catalog"}
          </h2>
          <p className={styles.muted} style={{ margin: "4px 0 0", fontSize: "var(--text-sm)" }}>
            {tab === "listings"
              ? "Raw asset listings and custom symbol mappings discovered from connected exchanges."
              : "Futures and derivatives contracts metadata discovered from connected exchanges."}
          </p>
        </div>
      </div>
      <form
        className={styles.compactFilterBar}
        style={{ marginBottom: "var(--space-md)" }}
        onSubmit={submitSearch}
      >
        <label className={styles.compactFilterField}>
          <span className={styles.compactFilterLabel}>Platform</span>
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
          <label className={styles.compactFilterField}>
            <span className={styles.compactFilterLabel}>Treatment</span>
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
          className={styles.compactFilterField}
          style={{ flex: "1 1 200px", maxWidth: "320px" }}
        >
          <span className={styles.compactFilterLabel}>Search</span>
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

interface TabProps {
  portfolioId: string;
  platform: string;
  q: string;
  ignoredOnly?: boolean;
  offset: number;
  timezone?: string;
  onOffset: (offset: number) => void;
}

function ListingsTab({
  portfolioId,
  platform,
  q,
  ignoredOnly,
  offset,
  timezone,
  onOffset,
}: TabProps) {
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const platforms = usePlatforms();
  const nonce = useRef(0);
  const [request, setRequest] = useState<MappingRequest | null>(null);
  const [exclusionRequest, setExclusionRequest] = useState<MappingRequest | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const query = useQuery({
    queryKey: ["catalog-listings", userId, portfolioId, platform, q, ignoredOnly, offset],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAssetListings({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            q: q || undefined,
            ignoredOnly: ignoredOnly || undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });

  async function invalidateListings() {
    await queryClient.invalidateQueries({ queryKey: ["catalog-listings", userId, portfolioId] });
  }

  return (
    <>
      {notice ? (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      ) : null}
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Asset listings could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <DataTable
        caption="Asset listings"
        columns={[
          { key: "platform", header: "Platform" },
          { key: "asset", header: "Asset" },
          { key: "treatment", header: "Treatment" },
          { key: "lastSeenAt", header: "Last seen", nowrap: true },
          { key: "actions", header: "", nowrap: true },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          platform: platformLabel(platforms.data?.items, item.platform),
          asset: (
            <span className={styles.cellStack}>
              <span
                style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-xs)" }}
              >
                <strong>{item.assetSymbol}</strong>
                {item.mappingSource && item.mappingSource !== "auto" ? (
                  <Badge tone="info">{item.mappingSource}</Badge>
                ) : null}
              </span>
              <span className={styles.cellMeta}>
                {item.name ? item.name : null}
                {item.symbol !== item.assetSymbol
                  ? `${item.name ? " · " : ""}platform: ${item.symbol}`
                  : null}
              </span>
            </span>
          ),
          treatment: item.ignored ? (
            <span className={styles.cellStack}>
              <StatusBadge value="ignored" />
              <span className={styles.cellMeta}>
                {item.ignoredAt
                  ? `Reviewed ${formatInstant(item.ignoredAt, timezone)}`
                  : "Reviewed"}
              </span>
              {item.ignoreReason ? (
                <details>
                  <summary>Reason</summary>
                  {item.ignoreReason}
                </details>
              ) : null}
            </span>
          ) : (
            "Included"
          ),
          lastSeenAt: formatInstant(item.lastSeenAt, timezone),
          actions: (
            <span className={styles.rowActions}>
              <ActionButton
                type="button"
                tone="ghost"
                onClick={() => {
                  setNotice(null);
                  nonce.current += 1;
                  setRequest({ nonce: nonce.current, item });
                }}
              >
                Map
              </ActionButton>
              <ActionButton
                type="button"
                tone="ghost"
                onClick={() => {
                  setNotice(null);
                  nonce.current += 1;
                  setExclusionRequest({ nonce: nonce.current, item });
                }}
              >
                {item.ignored ? "Restore" : "Ignore"}
              </ActionButton>
            </span>
          ),
        }))}
        empty={
          query.isPending
            ? "Loading asset listings."
            : q || platform || ignoredOnly
              ? "No listing matches these filters."
              : "No asset listings yet. Listings appear after the first Collection Cycle reports balances."
        }
      />
      <Pager
        offset={query.data?.page.offset ?? offset}
        limit={query.data?.page.limit ?? PAGE_SIZE}
        total={query.data?.page.total ?? 0}
        onChange={onOffset}
      />
      <Dialog
        open={request !== null}
        title={request ? `Map ${request.item.symbol}` : "Map listing"}
        onClose={() => setRequest(null)}
      >
        {request ? (
          <MappingForm
            key={request.nonce}
            portfolioId={portfolioId}
            item={request.item}
            onClose={() => setRequest(null)}
            onDone={async (message) => {
              setRequest(null);
              setNotice(message);
              await invalidateListings();
              await invalidatePortfolioReads(queryClient, userId, portfolioId);
            }}
          />
        ) : null}
      </Dialog>
      <Dialog
        open={exclusionRequest !== null}
        title={
          exclusionRequest
            ? `${exclusionRequest.item.ignored ? "Restore" : "Ignore"} ${exclusionRequest.item.symbol}`
            : "Listing treatment"
        }
        onClose={() => setExclusionRequest(null)}
      >
        {exclusionRequest ? (
          <ExclusionForm
            key={exclusionRequest.nonce}
            portfolioId={portfolioId}
            item={exclusionRequest.item}
            onClose={() => setExclusionRequest(null)}
            onDone={async (message) => {
              setExclusionRequest(null);
              setNotice(message);
              await invalidateListings();
              await invalidatePortfolioReads(queryClient, userId, portfolioId);
            }}
          />
        ) : null}
      </Dialog>
    </>
  );
}

function ExclusionForm({
  portfolioId,
  item,
  onClose,
  onDone,
}: {
  portfolioId: string;
  item: AssetListing;
  onClose: () => void;
  onDone: (message: string) => Promise<void>;
}) {
  const { redirect } = useSessionScope();
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const ignoring = !item.ignored;

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (ignoring && !reason.trim()) {
      setError("Explain why this balance can be excluded.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await putAssetListingExclusion({
        path: { portfolioId, listingId: item.id },
        headers: { "If-Match": exclusionEtag(item) },
        body: { ignored: ignoring, ...(ignoring ? { reason: reason.trim() } : {}) },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onDone(
          "The listing treatment changed in another session. The list was reloaded; open it again.",
        );
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The listing treatment could not be saved."));
        return;
      }
      await onDone(
        `${result.data.platform} ${result.data.symbol} ${ignoring ? "will be ignored" : "will be included"} from the next Collection Cycle. Use Collect now on Overview to update the latest value sooner.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The listing treatment could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={styles.lead}>
        {ignoring ? "Ignoring" : "Restoring"} {item.platform} {item.symbol} affects this Portfolio
        from the next Collection Cycle. Only Accounts calculated from Balances change; Accounts
        using Platform reported equity keep this Balance and their reported total. Earlier Valuation
        Points remain unchanged.
      </p>
      {item.ignored && item.ignoreReason ? <p>Review reason: {item.ignoreReason}</p> : null}
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {ignoring ? (
        <label className={styles.field}>
          <span>Review reason</span>
          <TextArea
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            maxLength={500}
            autoFocus
            required
          />
        </label>
      ) : null}
      <div className={styles.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          {ignoring ? "Ignore listing" : "Restore listing"}
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
    </form>
  );
}

function MappingForm({
  portfolioId,
  item,
  onClose,
  onDone,
}: {
  portfolioId: string;
  item: AssetListing;
  onClose: () => void;
  onDone: (message: string) => Promise<void>;
}) {
  const { redirect } = useSessionScope();
  const platforms = usePlatforms();
  const platform = platformLabel(platforms.data?.items, item.platform);
  const [assetSymbol, setAssetSymbol] = useState(item.assetSymbol);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    const symbol = assetSymbol.trim().toUpperCase();
    if (!symbol) {
      setError("Enter the Asset symbol this listing should merge into.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await putAssetListingMapping({
        path: { portfolioId, listingId: item.id },
        headers: { "If-Match": listingEtag(item) },
        body: { assetSymbol: symbol },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onDone(
          "The listing changed in another session. The list was reloaded; open it again.",
        );
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The mapping could not be saved."));
        return;
      }
      await onDone(
        `${platform} ${result.data.symbol} now maps to ${result.data.assetSymbol}. Valuations use it from the next Collection Cycle.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The mapping could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={styles.lead}>
        Assets merge by symbol across Platforms. Map {platform} {item.symbol}
        {item.name ? ` (${item.name})` : ""} to the symbol that names the same Asset elsewhere.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={styles.field}>
        <span>Asset symbol</span>
        <TextInput
          value={assetSymbol}
          onChange={(event) => setAssetSymbol(event.target.value)}
          maxLength={32}
          autoFocus
          required
        />
      </label>
      <div className={styles.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          Save mapping
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
    </form>
  );
}

function InstrumentsTab({ portfolioId, platform, q, offset, timezone, onOffset }: TabProps) {
  const { userId, redirect } = useSessionScope();
  const platforms = usePlatforms();
  const query = useQuery({
    queryKey: ["catalog-instruments", userId, portfolioId, platform, q, offset],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listInstruments({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            q: q || undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });

  return (
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Instruments could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <DataTable
        caption="Instruments"
        columns={[
          { key: "platform", header: "Platform" },
          { key: "symbol", header: "Symbol" },
          { key: "kind", header: "Kind" },
          { key: "pair", header: "Pair" },
          { key: "settlement", header: "Settlement" },
          { key: "expiryAt", header: "Expiry" },
          { key: "contractSize", header: "Contract size", numeric: true },
          { key: "lastSeenAt", header: "Last seen", nowrap: true },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          platform: platformLabel(platforms.data?.items, item.platform),
          symbol: item.symbol,
          kind: capitalize(item.kind.replaceAll("_", " ")),
          pair:
            item.baseSymbol || item.quoteSymbol
              ? `${item.baseSymbol ?? "—"} / ${item.quoteSymbol ?? "—"}`
              : "—",
          settlement: item.settlementSymbol ?? "—",
          expiryAt: item.expiryAt ? formatInstant(item.expiryAt, timezone) : "Perpetual",
          contractSize: formatDecimal(item.contractSize),
          lastSeenAt: formatInstant(item.lastSeenAt, timezone),
        }))}
        empty={
          query.isPending
            ? "Loading instruments."
            : q || platform
              ? "No instrument matches these filters."
              : "No instruments yet. Instruments appear after a Collection Cycle reports positions."
        }
      />
      <Pager
        offset={query.data?.page.offset ?? offset}
        limit={query.data?.page.limit ?? PAGE_SIZE}
        total={query.data?.page.total ?? 0}
        onChange={onOffset}
      />
    </>
  );
}
