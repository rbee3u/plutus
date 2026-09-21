import { type FormEvent, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  deleteCatalogPin,
  getOverview,
  putCatalogListingMapping,
  putCatalogPin,
  searchCatalogAssetListings,
  searchCatalogAssets,
  searchCatalogInstruments,
} from "@/api/generated/sdk.gen";
import type {
  CatalogAssetListing,
  CatalogInstrument,
  CatalogPinSubjectKind,
} from "@/api/generated/types.gen";
import { useProjectionFollow } from "@/features/data/useProjectionFollow";
import { PLATFORMS, platformLabel } from "@/features/data/platforms";
import { problemMessage, refreshCsrf } from "@/api/http";
import { newIdempotencyKey } from "@/shared/lib/id";
import { readPreference, storePreference } from "@/shared/lib/preferences";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Pager } from "@/shared/ui/Pager";
import { Select, TextArea, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

function pinStorageKey(
  portfolioId: string,
  kind: CatalogPinSubjectKind,
  subjectId: string,
): string {
  return `plutus-catalog-pin-etag:${portfolioId}:${kind}:${subjectId}`;
}

function pinEtag(
  portfolioId: string,
  kind: CatalogPinSubjectKind,
  subjectId: string,
  pinned: boolean,
): string {
  if (!pinned) {
    return "*";
  }
  return readPreference(pinStorageKey(portfolioId, kind, subjectId)) ?? `"catalog-pin:1"`;
}

function mappingEtag(listing: CatalogAssetListing): string {
  if (!listing.mappingVersion || listing.mappingState === "unmapped") {
    return "*";
  }
  return `"listing-mapping:${String(listing.mappingVersion)}"`;
}

export function CatalogPage() {
  const { portfolioId = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") === "instruments" ? "instruments" : "listings";
  const q = params.get("q") ?? "";
  const platformId = params.get("platformId") ?? "";
  const mappingState = params.get("mappingState") ?? "";
  const pinned = params.get("pinned") ?? "";
  const offset = Math.max(0, Number(params.get("offset") ?? "0") || 0);

  function update(next: Record<string, string>) {
    const merged = new URLSearchParams(params);
    for (const [key, value] of Object.entries(next)) {
      if (value) merged.set(key, value);
      else merged.delete(key);
    }
    if (!("offset" in next)) {
      merged.delete("offset");
    }
    setParams(merged, { replace: true });
  }

  return (
    <section className={styles.wide}>
      <h2>Catalog</h2>
      <p className={styles.lead}>
        Search discovered listings and instruments. Mapping and tracking revisions enqueue a full
        projection rebuild.
      </p>
      <div className={styles.actions}>
        <ActionButton
          type="button"
          tone={tab === "listings" ? "primary" : "secondary"}
          onClick={() => update({ tab: "listings" })}
        >
          Listings
        </ActionButton>
        <ActionButton
          type="button"
          tone={tab === "instruments" ? "primary" : "secondary"}
          onClick={() => update({ tab: "instruments" })}
        >
          Instruments
        </ActionButton>
      </div>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>Search</span>
          <TextInput value={q} onChange={(event) => update({ q: event.target.value })} />
        </label>
        <label className={styles.field}>
          <span>Platform</span>
          <Select
            value={platformId}
            onChange={(event) => update({ platformId: event.target.value })}
          >
            <option value="">All</option>
            {PLATFORMS.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </label>
        {tab === "listings" ? (
          <label className={styles.field}>
            <span>Mapping</span>
            <Select
              value={mappingState}
              onChange={(event) => update({ mappingState: event.target.value })}
            >
              <option value="">All</option>
              <option value="unmapped">Unmapped</option>
              <option value="provisional">Provisional</option>
              <option value="confirmed">Confirmed</option>
            </Select>
          </label>
        ) : (
          <label className={styles.field}>
            <span>Pinned</span>
            <Select value={pinned} onChange={(event) => update({ pinned: event.target.value })}>
              <option value="">All</option>
              <option value="true">Pinned</option>
              <option value="false">Unpinned</option>
            </Select>
          </label>
        )}
      </form>
      {tab === "listings" ? (
        <ListingResults
          key={`${q}|${platformId}|${mappingState}|${offset}`}
          portfolioId={portfolioId}
          q={q}
          platformId={platformId}
          mappingState={mappingState}
          offset={offset}
          onOffset={(nextOffset) => update({ offset: nextOffset > 0 ? String(nextOffset) : "" })}
        />
      ) : (
        <InstrumentResults
          key={`${q}|${platformId}|${pinned}|${offset}`}
          portfolioId={portfolioId}
          q={q}
          platformId={platformId}
          pinned={pinned}
          offset={offset}
          onOffset={(nextOffset) => update({ offset: nextOffset > 0 ? String(nextOffset) : "" })}
        />
      )}
    </section>
  );
}

function ListingResults({
  portfolioId,
  q,
  platformId,
  mappingState,
  offset,
  onOffset,
}: {
  portfolioId: string;
  q: string;
  platformId: string;
  mappingState: string;
  offset: number;
  onOffset: (offset: number) => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const follow = useProjectionFollow(portfolioId);
  const mappingKey = useRef(newIdempotencyKey());
  const pinKey = useRef(newIdempotencyKey());
  const [mappingTarget, setMappingTarget] = useState<CatalogAssetListing | null>(null);
  const [pinTarget, setPinTarget] = useState<CatalogAssetListing | null>(null);
  const [assetId, setAssetId] = useState("");
  const [state, setState] = useState<"provisional" | "confirmed">("provisional");
  const [reason, setReason] = useState("");
  const [override, setOverride] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const listings = useQuery({
    queryKey: ["catalog-listings", user.data?.id, portfolioId, q, platformId, mappingState, offset],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await searchCatalogAssetListings({
          path: { portfolioId },
          query: {
            q: q || undefined,
            platformId: platformId || undefined,
            mappingState:
              mappingState === "unmapped" ||
              mappingState === "provisional" ||
              mappingState === "confirmed"
                ? mappingState
                : undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });
  const assets = useQuery({
    queryKey: ["catalog-assets", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await searchCatalogAssets({ path: { portfolioId }, query: { limit: 200 } }),
        redirect,
      ),
  });

  async function snapshotGeneration(): Promise<string> {
    const overview = await queryClient.fetchQuery({
      queryKey: ["overview", user.data?.id, portfolioId],
      queryFn: async () => unwrapOrRedirect(await getOverview({ path: { portfolioId } }), redirect),
    });
    return overview.context.generationId ?? "";
  }

  async function saveMapping(event: FormEvent) {
    event.preventDefault();
    if (!mappingTarget) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const generationId = await snapshotGeneration();
      await refreshCsrf();
      const result = await putCatalogListingMapping({
        path: { portfolioId, listingId: mappingTarget.id },
        headers: {
          "If-Match": mappingEtag(mappingTarget),
          "Idempotency-Key": mappingKey.current,
        },
        body: {
          assetId,
          state,
          displayNameOverride: override.trim() ? override : null,
          reason: reason.trim() ? reason : null,
        },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await queryClient.invalidateQueries({
          queryKey: ["catalog-listings", user.data?.id, portfolioId],
        });
        setError("The mapping changed in another session. Search again, then retry.");
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The mapping could not be saved."));
        return;
      }
      mappingKey.current = newIdempotencyKey();
      follow.follow(result.data.job, generationId);
      setMappingTarget(null);
      await queryClient.invalidateQueries({
        queryKey: ["catalog-listings", user.data?.id, portfolioId],
      });
    } catch (cause) {
      setError(problemMessage(cause, "The mapping could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  async function savePin(event: FormEvent) {
    event.preventDefault();
    if (!pinTarget) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const generationId = await snapshotGeneration();
      await refreshCsrf();
      const match = pinEtag(portfolioId, "listing", pinTarget.id, Boolean(pinTarget.pinned));
      const result = pinTarget.pinned
        ? await deleteCatalogPin({
            path: { portfolioId, subjectKind: "listing", subjectId: pinTarget.id },
            headers: { "If-Match": match },
          })
        : await putCatalogPin({
            path: { portfolioId, subjectKind: "listing", subjectId: pinTarget.id },
            headers: { "If-Match": match, "Idempotency-Key": pinKey.current },
            body: { reason: reason.trim() ? reason : null },
          });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setError("The pin changed in another session. Search again, then retry.");
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The tracking revision could not be saved."));
        return;
      }
      pinKey.current = newIdempotencyKey();
      storePreference(
        pinStorageKey(portfolioId, "listing", pinTarget.id),
        `"catalog-pin:${String(result.data.pin.version)}"`,
      );
      follow.follow(result.data.job, generationId);
      setPinTarget(null);
      await queryClient.invalidateQueries({
        queryKey: ["catalog-listings", user.data?.id, portfolioId],
      });
    } catch (cause) {
      setError(problemMessage(cause, "The tracking revision could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <QueryRecovery
        error={listings.error}
        hasData={Boolean(listings.data)}
        busy={listings.isFetching}
        fallback="Listings could not be loaded."
        onRetry={() => {
          void listings.refetch();
        }}
      />
      {follow.notice ? <p className={styles.notice}>{follow.notice}</p> : null}
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <DataTable
        caption="Asset listings"
        columns={[
          { key: "symbol", header: "Symbol" },
          { key: "platform", header: "Platform" },
          { key: "asset", header: "Mapped asset" },
          { key: "state", header: "Mapping" },
          { key: "pinned", header: "Pinned" },
        ]}
        rows={(listings.data?.items ?? []).map((item) => ({
          id: item.id,
          symbol: item.symbol,
          platform: item.platformCode ?? platformLabel(item.platformId),
          asset: item.mappedAssetCode ?? "—",
          state: item.mappingState ?? "unmapped",
          pinned: item.pinned ? "yes" : "no",
        }))}
      />
      <ul className={styles.flowActions}>
        {(listings.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>{item.symbol}</span>
            <span className={styles.actions}>
              <ActionButton
                type="button"
                disabled={busy}
                onClick={() => {
                  setError(null);
                  setMappingTarget(item);
                  setAssetId(item.mappedAssetId ?? "");
                  setState(item.mappingState === "confirmed" ? "confirmed" : "provisional");
                  setOverride("");
                  setReason("");
                }}
              >
                Map
              </ActionButton>
              <ActionButton
                type="button"
                disabled={busy}
                onClick={() => {
                  setError(null);
                  setPinTarget(item);
                  setReason("");
                }}
              >
                {item.pinned ? "Unpin" : "Pin"}
              </ActionButton>
            </span>
          </li>
        ))}
      </ul>
      <Pager
        offset={offset}
        limit={PAGE_SIZE}
        total={listings.data?.page.total ?? 0}
        onChange={onOffset}
      />
      <Dialog
        open={Boolean(mappingTarget)}
        title="Mapping revision"
        onClose={() => setMappingTarget(null)}
      >
        <form className={styles.form} onSubmit={(event) => void saveMapping(event)}>
          <label className={styles.field}>
            <span>Asset</span>
            <Select value={assetId} onChange={(event) => setAssetId(event.target.value)} required>
              <option value="" disabled>
                Select an asset
              </option>
              {(assets.data?.items ?? []).map((item) => (
                <option key={item.id} value={item.id}>
                  {item.code}
                </option>
              ))}
            </Select>
          </label>
          <label className={styles.field}>
            <span>State</span>
            <Select
              value={state}
              onChange={(event) => setState(event.target.value as "provisional" | "confirmed")}
            >
              <option value="provisional">Provisional</option>
              <option value="confirmed">Confirmed</option>
            </Select>
          </label>
          <label className={styles.field}>
            <span>Display name override</span>
            <TextInput value={override} onChange={(event) => setOverride(event.target.value)} />
          </label>
          <label className={styles.field}>
            <span>Reason</span>
            <TextArea value={reason} onChange={(event) => setReason(event.target.value)} rows={3} />
          </label>
          <div className={styles.actions}>
            <ActionButton type="submit" tone="primary" disabled={busy}>
              Save mapping
            </ActionButton>
            <ActionButton type="button" onClick={() => setMappingTarget(null)}>
              Cancel
            </ActionButton>
          </div>
        </form>
      </Dialog>
      <Dialog
        open={Boolean(pinTarget)}
        title={pinTarget?.pinned ? "Disable tracking" : "Pin listing"}
        onClose={() => setPinTarget(null)}
      >
        <form className={styles.form} onSubmit={(event) => void savePin(event)}>
          <label className={styles.field}>
            <span>Reason</span>
            <TextArea value={reason} onChange={(event) => setReason(event.target.value)} rows={3} />
          </label>
          <div className={styles.actions}>
            <ActionButton type="submit" tone="primary" disabled={busy}>
              {pinTarget?.pinned ? "Unpin" : "Pin"}
            </ActionButton>
            <ActionButton type="button" onClick={() => setPinTarget(null)}>
              Cancel
            </ActionButton>
          </div>
        </form>
      </Dialog>
    </>
  );
}

function InstrumentResults({
  portfolioId,
  q,
  platformId,
  pinned,
  offset,
  onOffset,
}: {
  portfolioId: string;
  q: string;
  platformId: string;
  pinned: string;
  offset: number;
  onOffset: (offset: number) => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const follow = useProjectionFollow(portfolioId);
  const pinKey = useRef(newIdempotencyKey());
  const [pinTarget, setPinTarget] = useState<CatalogInstrument | null>(null);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const instruments = useQuery({
    queryKey: ["catalog-instruments", user.data?.id, portfolioId, q, platformId, pinned, offset],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await searchCatalogInstruments({
          path: { portfolioId },
          query: {
            q: q || undefined,
            platformId: platformId || undefined,
            pinned: pinned === "true" ? true : pinned === "false" ? false : undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });

  async function savePin(event: FormEvent) {
    event.preventDefault();
    if (!pinTarget) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const overview = await queryClient.fetchQuery({
        queryKey: ["overview", user.data?.id, portfolioId],
        queryFn: async () =>
          unwrapOrRedirect(await getOverview({ path: { portfolioId } }), redirect),
      });
      await refreshCsrf();
      const match = pinEtag(portfolioId, "instrument", pinTarget.id, Boolean(pinTarget.pinned));
      const result = pinTarget.pinned
        ? await deleteCatalogPin({
            path: { portfolioId, subjectKind: "instrument", subjectId: pinTarget.id },
            headers: { "If-Match": match },
          })
        : await putCatalogPin({
            path: { portfolioId, subjectKind: "instrument", subjectId: pinTarget.id },
            headers: { "If-Match": match, "Idempotency-Key": pinKey.current },
            body: { reason: reason.trim() ? reason : null },
          });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setError("The pin changed in another session. Search again, then retry.");
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The tracking revision could not be saved."));
        return;
      }
      pinKey.current = newIdempotencyKey();
      storePreference(
        pinStorageKey(portfolioId, "instrument", pinTarget.id),
        `"catalog-pin:${String(result.data.pin.version)}"`,
      );
      follow.follow(result.data.job, overview.context.generationId);
      setPinTarget(null);
      await queryClient.invalidateQueries({
        queryKey: ["catalog-instruments", user.data?.id, portfolioId],
      });
    } catch (cause) {
      setError(problemMessage(cause, "The tracking revision could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <QueryRecovery
        error={instruments.error}
        hasData={Boolean(instruments.data)}
        busy={instruments.isFetching}
        fallback="Instruments could not be loaded."
        onRetry={() => {
          void instruments.refetch();
        }}
      />
      {follow.notice ? <p className={styles.notice}>{follow.notice}</p> : null}
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <DataTable
        caption="Instruments"
        columns={[
          { key: "symbol", header: "Symbol" },
          { key: "platform", header: "Platform" },
          { key: "kind", header: "Kind" },
          { key: "status", header: "Status" },
          { key: "pinned", header: "Pinned" },
        ]}
        rows={(instruments.data?.items ?? []).map((item) => ({
          id: item.id,
          symbol: item.symbol,
          platform: item.platformCode ?? platformLabel(item.platformId),
          kind: item.kind,
          status: item.status,
          pinned: item.pinned ? "yes" : "no",
        }))}
      />
      <ul className={styles.flowActions}>
        {(instruments.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>{item.symbol}</span>
            <span className={styles.actions}>
              <ActionButton
                type="button"
                disabled={busy}
                onClick={() => {
                  setError(null);
                  setPinTarget(item);
                  setReason("");
                }}
              >
                {item.pinned ? "Unpin" : "Pin"}
              </ActionButton>
            </span>
          </li>
        ))}
      </ul>
      <Pager
        offset={offset}
        limit={PAGE_SIZE}
        total={instruments.data?.page.total ?? 0}
        onChange={onOffset}
      />
      <Dialog
        open={Boolean(pinTarget)}
        title={pinTarget?.pinned ? "Disable tracking" : "Pin instrument"}
        onClose={() => setPinTarget(null)}
      >
        <form className={styles.form} onSubmit={(event) => void savePin(event)}>
          <label className={styles.field}>
            <span>Reason</span>
            <TextArea value={reason} onChange={(event) => setReason(event.target.value)} rows={3} />
          </label>
          <div className={styles.actions}>
            <ActionButton type="submit" tone="primary" disabled={busy}>
              {pinTarget?.pinned ? "Unpin" : "Pin"}
            </ActionButton>
            <ActionButton type="button" onClick={() => setPinTarget(null)}>
              Cancel
            </ActionButton>
          </div>
        </form>
      </Dialog>
    </>
  );
}
