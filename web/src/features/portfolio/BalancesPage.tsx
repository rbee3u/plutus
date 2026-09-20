import {
  listAccounts,
  listBalances,
  listIdentities,
  searchCatalogAssets,
} from "@/api/generated/sdk.gen";
import type { BalanceList } from "@/api/generated/types.gen";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

const PAGE_SIZE = 50;
const GROUP_OPTIONS = ["", "platform", "identity", "account", "asset", "listing"] as const;

export function BalancesPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const user = useCurrentUser();
  const overview = useOverview(portfolioId);
  const [params, setParams] = useSearchParams();
  const groupBy = readGroup(params.get("groupBy"));
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const assetId = params.get("assetId") ?? "";
  const listingId = params.get("listingId") ?? "";
  const offset = Math.max(0, Number(params.get("offset") ?? "0") || 0);
  const generationId = overview.data?.context.generationId ?? "";
  const redirect = () => {
    void navigate("/login", { replace: true });
  };

  const identities = useQuery({
    queryKey: ["identities", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await listIdentities({ path: { portfolioId } }), redirect),
  });
  const accounts = useQuery({
    queryKey: ["accounts", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAccounts({ path: { portfolioId }, query: { limit: 200 } }),
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
  const query = useQuery({
    queryKey: [
      "balances",
      user.data?.id,
      portfolioId,
      generationId,
      groupBy,
      identityId,
      accountId,
      assetId,
      listingId,
      offset,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<BalanceList> =>
      unwrapOrRedirect(
        await listBalances({
          path: { portfolioId },
          query: {
            groupBy: groupBy || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
            assetId: assetId || undefined,
            listingId: listingId || undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });

  const rows = useMemo(
    () =>
      (query.data?.items ?? []).map((item) => ({
        id: `${item.accountId}-${item.assetListingId}`,
        identity: item.identityName ?? item.identityId,
        account: item.accountName ?? item.accountId,
        platform: item.platformCode ?? "—",
        listing: item.listingSymbol ?? item.assetListingId,
        asset: item.assetCode ?? "—",
        total: formatDecimal(item.totalQuantity),
        available: formatDecimal(item.availableQuantity),
        locked: formatDecimal(item.lockedQuantity),
        observed: formatInstant(item.sourceObservedAt, query.data?.context.timezone),
        freshness: item.freshness,
        completeness: item.completeness,
      })),
    [query.data],
  );

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
      <h2>Balances</h2>
      {query.error ? (
        <p className={styles.alert} role="alert">
          {query.error instanceof Error ? query.error.message : "Balances could not be loaded."}
        </p>
      ) : null}
      {query.data ? <ContextSummary context={query.data.context} /> : null}
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>Group by</span>
          <Select value={groupBy} onChange={(event) => update({ groupBy: event.target.value })}>
            {GROUP_OPTIONS.map((option) => (
              <option key={option || "none"} value={option}>
                {option || "No grouping"}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Identity</span>
          <Select
            value={identityId}
            onChange={(event) => update({ identityId: event.target.value })}
          >
            <option value="">All</option>
            {(identities.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.displayName}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Account</span>
          <Select value={accountId} onChange={(event) => update({ accountId: event.target.value })}>
            <option value="">All</option>
            {(accounts.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.displayName}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Asset</span>
          <Select value={assetId} onChange={(event) => update({ assetId: event.target.value })}>
            <option value="">All</option>
            {(assets.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.code}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Listing id</span>
          <TextInput
            value={listingId}
            onChange={(event) => update({ listingId: event.target.value })}
          />
        </label>
      </form>
      {query.data && groupBy ? (
        <section className={styles.panel}>
          <h3>Group totals</h3>
          <DataTable
            caption="Server-calculated group totals"
            columns={[
              { key: "label", header: "Group" },
              { key: "total", header: "Total", numeric: true },
              { key: "available", header: "Available", numeric: true },
              { key: "locked", header: "Locked", numeric: true },
              { key: "completeness", header: "Completeness" },
            ]}
            rows={query.data.groups.map((group) => ({
              key: group.key,
              label: group.label,
              total: formatDecimal(group.totals.totalQuantity),
              available: formatDecimal(group.totals.availableQuantity),
              locked: formatDecimal(group.totals.lockedQuantity),
              completeness: group.completeness,
            }))}
          />
        </section>
      ) : null}
      <DataTable
        caption="Latest balances"
        columns={[
          { key: "identity", header: "Identity" },
          { key: "account", header: "Account" },
          { key: "platform", header: "Platform" },
          { key: "listing", header: "Listing" },
          { key: "asset", header: "Asset" },
          { key: "total", header: "Total", numeric: true },
          { key: "available", header: "Available", numeric: true },
          { key: "locked", header: "Locked", numeric: true },
          { key: "observed", header: "Observed" },
          { key: "freshness", header: "Freshness" },
          { key: "completeness", header: "Completeness" },
        ]}
        rows={rows}
      />
      <Pager
        offset={offset}
        limit={PAGE_SIZE}
        total={query.data?.page.total ?? 0}
        onChange={(nextOffset) => update({ offset: String(nextOffset) })}
      />
      {query.data ? (
        <section className={styles.panel}>
          <h3>Issues</h3>
          <IssueList issues={query.data.issues} />
        </section>
      ) : null}
    </section>
  );
}

function readGroup(value: string | null): (typeof GROUP_OPTIONS)[number] {
  return GROUP_OPTIONS.includes(value as (typeof GROUP_OPTIONS)[number])
    ? (value as (typeof GROUP_OPTIONS)[number])
    : "";
}
