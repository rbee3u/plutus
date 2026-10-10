import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useRef, useState } from "react";
import {
  invalidatePortfolioReads,
  keys,
  listAssets,
  problemMessage,
  putAssetExclusion,
  putAssetMapping,
} from "@/api";
import type { Asset, AssetList } from "@/api";
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
  assetEtag,
  type MappingRequest,
  type TabProps,
} from "../lib/catalog";
import shared from "@/shared/styles/page.module.css";
import manageStyles from "../styles.module.css";

export function AssetsTab({ portfolioId, filters, timezone }: TabProps) {
  const { userId } = useSessionScope();
  const queryClient = useQueryClient();
  const { platform, q, excludedOnly } = filters.values;
  const offset = filters.offset;
  const platforms = filters.platforms;
  const nonce = useRef(0);
  const [request, setRequest] = useState<MappingRequest | null>(null);
  const [exclusionRequest, setExclusionRequest] = useState<MappingRequest | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const query = usePortfolioQuery<AssetList>({
    key: (userId, portfolioId) =>
      keys.catalogAssets(userId, portfolioId, platform, q, excludedOnly, offset),
    load: (portfolioId) =>
      listAssets({
        path: { portfolioId },
        query: {
          platform: platform || undefined,
          q: q || undefined,
          excludedOnly: excludedOnly || undefined,
          offset,
          limit: PAGE_SIZE,
        },
      }),
  });

  async function invalidateAssets() {
    await queryClient.invalidateQueries({
      queryKey: keys.catalogAssets(userId, portfolioId),
    });
  }

  return (
    <>
      {notice ? (
        <p className={shared.notice} role="status">
          {notice}
        </p>
      ) : null}
      <QueryRecovery query={query} fallback="Assets could not be loaded." />
      <DataTable
        caption="Assets"
        columns={[
          {
            key: "platform",
            header: "Platform",
            nowrap: true,
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
                display={filters.qDraft}
                active={Boolean(q || filters.qDraft)}
                open={filters.open === "q"}
                type="search"
                searchDraft={filters.qDraft}
                searchPlaceholder="Filter asset or canonical symbol"
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
            key: "status",
            header: "Status",
            nowrap: true,
            headerContent: (
              <TableFilterHeader
                label="Status"
                display={filters.statusName}
                active={excludedOnly}
                open={filters.open === "status"}
                type="select"
                options={filters.statusOptions}
                selectedValue={excludedOnly ? "true" : ""}
                onToggle={() => filters.toggle("status")}
                onClose={filters.close}
                onClear={() => filters.change({ excludedOnly: false })}
                onSelect={(val) => filters.change({ excludedOnly: val === "true" })}
              />
            ),
          },
          { key: "lastObservedAt", header: "Last observed", nowrap: true },
          { key: "actions", header: "Actions", nowrap: true, numeric: true },
        ]}
        rows={(query.data?.items ?? []).map((item) => {
          const isMapped = item.symbol !== item.canonicalAssetSymbol;

          return {
            id: item.id,
            platform: platformLabel(platforms.data?.items, item.platform),
            asset: (
              <span className={shared.cellStack}>
                <span
                  style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-xs)" }}
                >
                  <strong style={{ letterSpacing: "0.02em" }}>{item.symbol}</strong>
                  {isMapped ? <Badge tone="info">mapped</Badge> : null}
                </span>
                {isMapped ? (
                  <span className={shared.cellMeta}>
                    Canonical Asset: {item.canonicalAssetSymbol}
                  </span>
                ) : null}
              </span>
            ),
            status: item.excluded ? (
              <span
                className={manageStyles.statusExcluded}
                title={
                  item.exclusionReason
                    ? `${item.exclusionReason}${item.excludedAt ? ` (${formatInstant(item.excludedAt, timezone)})` : ""}`
                    : item.excludedAt
                      ? `Excluded at ${formatInstant(item.excludedAt, timezone)}`
                      : "Excluded"
                }
              >
                <StatusBadge value="excluded" />
                {item.exclusionReason ? (
                  <span className={manageStyles.exclusionReason}>{item.exclusionReason}</span>
                ) : null}
              </span>
            ) : (
              <span className={shared.cellMeta}>Active</span>
            ),
            lastObservedAt: (
              <span className={shared.tabularDate}>
                {formatInstant(item.lastObservedAt, timezone)}
              </span>
            ),
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
                  {item.excluded ? "Restore" : "Ignore"}
                </ActionButton>
              </span>
            ),
          };
        })}
        empty={
          query.isPending ? (
            <div className={shared.emptyState}>
              <p className={shared.emptyStateTitle}>Loading assets...</p>
            </div>
          ) : q || platform || excludedOnly ? (
            <div className={shared.emptyState}>
              <p className={shared.emptyStateTitle}>No assets match these filters</p>
              <p className={shared.emptyStateHelp}>
                Try adjusting your search query, platform, or status filter.
              </p>
              <ActionButton type="button" tone="secondary" onClick={filters.reset}>
                Clear filters
              </ActionButton>
            </div>
          ) : (
            <div className={shared.emptyState}>
              <p className={shared.emptyStateTitle}>No assets recorded yet</p>
              <p className={shared.emptyStateHelp}>
                Assets will appear automatically once platform identities are connected and sync.
              </p>
            </div>
          )
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
        title={request ? `Map ${request.item.symbol}` : "Map asset"}
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
              await invalidateAssets();
              await invalidatePortfolioReads(queryClient, userId, portfolioId);
            }}
          />
        ) : null}
      </Dialog>
      <Dialog
        open={exclusionRequest !== null}
        title={
          exclusionRequest
            ? `${exclusionRequest.item.excluded ? "Restore" : "Ignore"} ${exclusionRequest.item.symbol}`
            : "Asset status"
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
              await invalidateAssets();
              await invalidatePortfolioReads(queryClient, userId, portfolioId);
            }}
          />
        ) : null}
      </Dialog>
    </>
  );
}

const EXCLUSION_REASON_PRESETS = [
  "Dust balance",
  "Spam / Airdrop",
  "Delisted asset",
  "Duplicate asset",
];

function ExclusionForm({
  portfolioId,
  item,
  onClose,
  onDone,
}: {
  portfolioId: string;
  item: Asset;
  onClose: () => void;
  onDone: (message: string) => Promise<void>;
}) {
  const write = useAuthenticatedWrite();
  const platforms = usePlatforms();
  const platform = platformLabel(platforms.data?.items, item.platform);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const ignoring = !item.excluded;

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
        putAssetExclusion({
          path: { portfolioId, assetId: item.id },
          headers: { "If-Match": exclusionEtag(item) },
          body: { excluded: ignoring, ...(ignoring ? { reason: reason.trim() } : {}) },
        }),
      );
      if (result.response?.status === 412) {
        await onDone(
          "The asset status changed in another session. The list was reloaded; open it again.",
        );
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The asset status could not be saved."));
        return;
      }
      await onDone(
        `${platform} ${result.data.symbol} ${ignoring ? "will be excluded" : "will be included"} from the next Collection Cycle. Use Collect now on Overview to update the latest value sooner.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The asset status could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={shared.lead}>
        {ignoring
          ? `Exclude ${platform} · ${item.symbol} from portfolio balances and valuations starting from the next cycle.`
          : `Include ${platform} · ${item.symbol} in portfolio balances and valuations starting from the next cycle.`}
      </p>
      {item.excluded && item.exclusionReason ? (
        <div className={manageStyles.reasonCallout}>
          <span className={manageStyles.reasonCalloutLabel}>Current exclusion reason</span>
          <span className={manageStyles.reasonCalloutText}>“{item.exclusionReason}”</span>
        </div>
      ) : null}
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      {ignoring ? (
        <div className={shared.field}>
          <span>Review reason</span>
          <div className={manageStyles.presetChips}>
            <span className={manageStyles.presetLabel}>Quick reasons:</span>
            {EXCLUSION_REASON_PRESETS.map((preset) => (
              <button
                key={preset}
                type="button"
                className={manageStyles.presetChip}
                onClick={() => {
                  setReason(preset);
                  setError(null);
                }}
              >
                {preset}
              </button>
            ))}
          </div>
          <TextArea
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="Select a quick reason above or enter custom review notes..."
            maxLength={500}
            rows={3}
            autoFocus
            required
          />
        </div>
      ) : null}
      <div className={shared.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          {ignoring ? "Exclude asset" : "Restore asset"}
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
  item: Asset;
  onClose: () => void;
  onDone: (message: string) => Promise<void>;
}) {
  const write = useAuthenticatedWrite();
  const platforms = usePlatforms();
  const platform = platformLabel(platforms.data?.items, item.platform);
  const [canonicalAssetSymbol, setCanonicalAssetSymbol] = useState(item.canonicalAssetSymbol);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const trimmedSymbol = canonicalAssetSymbol.trim().toUpperCase();
  const isUnchanged = trimmedSymbol === item.canonicalAssetSymbol;
  const isCustomMapped = item.symbol !== item.canonicalAssetSymbol;

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!trimmedSymbol) {
      setError("Enter the Canonical Asset symbol this Asset maps to.");
      return;
    }
    if (
      new TextEncoder().encode(trimmedSymbol).length > 64 ||
      !/^[\p{L}\p{N}][\p{L}\p{N}._-]*$/u.test(trimmedSymbol)
    ) {
      setError("Use letters, numbers, dots, underscores or hyphens, up to 64 UTF-8 bytes.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        putAssetMapping({
          path: { portfolioId, assetId: item.id },
          headers: { "If-Match": assetEtag(item) },
          body: { canonicalAssetSymbol: trimmedSymbol },
        }),
      );
      if (result.response?.status === 412) {
        await onDone("The asset changed in another session. The list was reloaded; open it again.");
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The mapping could not be saved."));
        return;
      }
      await onDone(
        `${platform} ${result.data.symbol} now maps to ${result.data.canonicalAssetSymbol}. Valuations use it from the next Collection Cycle.`,
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
        Map {platform} · {item.symbol} to a Canonical Asset symbol across Platforms.
      </p>
      <div className={manageStyles.mappingFlowCard}>
        <div className={manageStyles.mappingFlowSource}>
          <span className={manageStyles.mappingFlowLabel}>Asset</span>
          <span className={manageStyles.mappingFlowSymbol}>{item.symbol}</span>
          <span className={manageStyles.mappingFlowPlatform}>{platform}</span>
        </div>
        <div className={manageStyles.mappingFlowArrow} aria-hidden="true">
          ➔
        </div>
        <div className={manageStyles.mappingFlowTarget}>
          <span className={manageStyles.mappingFlowLabel}>Canonical Asset</span>
          <span className={manageStyles.mappingFlowSymbol}>{trimmedSymbol || "—"}</span>
          <span className={manageStyles.mappingFlowPlatform}>
            {isCustomMapped && trimmedSymbol !== item.symbol
              ? "Custom mapped"
              : isUnchanged
                ? "Native (1:1)"
                : "New target"}
          </span>
        </div>
      </div>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={shared.field}>
        <span>Canonical Asset</span>
        <TextInput
          value={canonicalAssetSymbol}
          onChange={(event) => setCanonicalAssetSymbol(event.target.value.toUpperCase())}
          maxLength={64}
          autoFocus
          autoCapitalize="characters"
          autoCorrect="off"
          spellCheck={false}
          style={{
            fontFamily: "var(--font-mono)",
            fontWeight: "var(--weight-semibold)",
            letterSpacing: "0.04em",
          }}
          required
        />
      </label>
      {isCustomMapped && trimmedSymbol !== item.symbol ? (
        <p className={shared.cellMeta}>
          Mapped from native {item.symbol}.{" "}
          <ActionButton
            type="button"
            tone="ghost"
            onClick={() => setCanonicalAssetSymbol(item.symbol)}
          >
            Reset to native ({item.symbol})
          </ActionButton>
        </p>
      ) : null}
      <div className={shared.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy || isUnchanged}>
          Save mapping
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
        {isUnchanged ? (
          <span className={shared.cellMeta} style={{ marginInlineStart: "auto" }}>
            Unchanged
          </span>
        ) : null}
      </div>
    </form>
  );
}
