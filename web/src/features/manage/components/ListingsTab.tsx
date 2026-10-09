import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useRef, useState } from "react";
import {
  invalidatePortfolioReads,
  keys,
  listAssetListings,
  problemMessage,
  putAssetListingExclusion,
  putAssetListingMapping,
} from "@/api";
import type { AssetListing, AssetListingList } from "@/api";
import { formatInstant } from "@/shared/lib/datetime";
import { useSessionScope } from "@/shared/lib/session";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Badge, StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { TextArea, TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";
import { TableFilterHeader } from "@/shared/ui/TableFilterHeader";
import {
  PAGE_SIZE,
  exclusionEtag,
  listingEtag,
  type MappingRequest,
  type TabProps,
} from "../lib/catalog";
import shared from "@/shared/styles/page.module.css";

export function ListingsTab({ portfolioId, filters, timezone }: TabProps) {
  const { userId } = useSessionScope();
  const queryClient = useQueryClient();
  const { platform, q, ignoredOnly } = filters.values;
  const offset = filters.offset;
  const platforms = filters.platforms;
  const nonce = useRef(0);
  const [request, setRequest] = useState<MappingRequest | null>(null);
  const [exclusionRequest, setExclusionRequest] = useState<MappingRequest | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const query = usePortfolioQuery<AssetListingList>({
    key: (userId, portfolioId) =>
      keys.catalogListings(userId, portfolioId, platform, q, ignoredOnly, offset),
    load: (portfolioId) =>
      listAssetListings({
        path: { portfolioId },
        query: {
          platform: platform || undefined,
          q: q || undefined,
          ignoredOnly: ignoredOnly || undefined,
          offset,
          limit: PAGE_SIZE,
        },
      }),
  });

  async function invalidateListings() {
    await queryClient.invalidateQueries({ queryKey: keys.catalogListings(userId, portfolioId) });
  }

  return (
    <>
      {notice ? (
        <p className={shared.notice} role="status">
          {notice}
        </p>
      ) : null}
      <QueryRecovery query={query} fallback="Asset listings could not be loaded." />
      <DataTable
        caption="Asset listings"
        columns={[
          {
            key: "platform",
            header: "Platform",
            headerContent: (
              <TableFilterHeader
                label="Platform"
                display={platform ? filters.platformName : "All"}
                active={Boolean(platform)}
                open={filters.open === "platform"}
                type="select"
                options={filters.platformOptions}
                selectedValue={platform}
                onToggle={() => filters.toggle("platform")}
                onClose={filters.close}
                onClear={() => filters.change({ platform: "" })}
                onSelect={(val) => filters.change({ platform: val })}
              />
            ),
          },
          {
            key: "asset",
            header: "Asset",
            headerContent: (
              <TableFilterHeader
                label="Asset"
                display={filters.qDraft || "All"}
                active={Boolean(q || filters.qDraft)}
                open={filters.open === "q"}
                type="search"
                searchDraft={filters.qDraft}
                searchPlaceholder="Symbol or name, e.g. BTC"
                searchHelp="Matches symbol or asset name, ignoring case."
                onToggle={() => filters.toggle("q")}
                onClose={filters.close}
                onClear={() => filters.change({ q: "" })}
                onSearchChange={filters.editQuery}
                onCompositionStart={filters.beginComposition}
                onCompositionEnd={filters.endComposition}
              />
            ),
          },
          {
            key: "treatment",
            header: "Treatment",
            headerContent: (
              <TableFilterHeader
                label="Treatment"
                display={filters.treatmentName}
                active={ignoredOnly}
                open={filters.open === "treatment"}
                type="select"
                options={filters.treatmentOptions}
                selectedValue={ignoredOnly ? "true" : ""}
                onToggle={() => filters.toggle("treatment")}
                onClose={filters.close}
                onClear={() => filters.change({ ignoredOnly: false })}
                onSelect={(val) => filters.change({ ignoredOnly: val === "true" })}
              />
            ),
          },
          { key: "lastSeenAt", header: "Last seen", nowrap: true },
          { key: "actions", header: "", nowrap: true },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          platform: platformLabel(platforms.data?.items, item.platform),
          asset: (
            <span className={shared.cellStack}>
              <span
                style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-xs)" }}
              >
                <strong>{item.assetSymbol}</strong>
                {item.mappingSource && item.mappingSource !== "auto" ? (
                  <Badge tone="info">{item.mappingSource}</Badge>
                ) : null}
              </span>
              <span className={shared.cellMeta}>
                {item.name ? item.name : null}
                {item.symbol !== item.assetSymbol
                  ? `${item.name ? " · " : ""}platform: ${item.symbol}`
                  : null}
              </span>
            </span>
          ),
          treatment: item.ignored ? (
            <span className={shared.cellStack}>
              <StatusBadge value="ignored" />
              <span className={shared.cellMeta}>
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
            <span className={shared.rowActions}>
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
      <TableFilterFooter
        page={query.data?.page}
        onPageChange={filters.page}
        hasFilters={filters.hasFilters}
        onReset={filters.reset}
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
  const write = useAuthenticatedWrite();
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
      const result = await write(() =>
        putAssetListingExclusion({
          path: { portfolioId, listingId: item.id },
          headers: { "If-Match": exclusionEtag(item) },
          body: { ignored: ignoring, ...(ignoring ? { reason: reason.trim() } : {}) },
        }),
      );
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
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={shared.lead}>
        {ignoring ? "Ignoring" : "Restoring"} {item.platform} {item.symbol} affects this Portfolio
        from the next Collection Cycle. Only Accounts calculated from Balances change; Accounts
        using Platform reported equity keep this Balance and their reported total. Earlier Valuation
        Points remain unchanged.
      </p>
      {item.ignored && item.ignoreReason ? <p>Review reason: {item.ignoreReason}</p> : null}
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      {ignoring ? (
        <label className={shared.field}>
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
      <div className={shared.actions}>
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
  const write = useAuthenticatedWrite();
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
      const result = await write(() =>
        putAssetListingMapping({
          path: { portfolioId, listingId: item.id },
          headers: { "If-Match": listingEtag(item) },
          body: { assetSymbol: symbol },
        }),
      );
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
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={shared.lead}>
        Assets merge by symbol across Platforms. Map {platform} {item.symbol}
        {item.name ? ` (${item.name})` : ""} to the symbol that names the same Asset elsewhere.
      </p>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={shared.field}>
        <span>Asset symbol</span>
        <TextInput
          value={assetSymbol}
          onChange={(event) => setAssetSymbol(event.target.value)}
          maxLength={32}
          autoFocus
          required
        />
      </label>
      <div className={shared.actions}>
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
