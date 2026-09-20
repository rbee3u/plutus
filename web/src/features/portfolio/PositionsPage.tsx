import { listAccounts, listIdentities, listPositions } from "@/api/generated/sdk.gen";
import type { PositionList } from "@/api/generated/types.gen";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Pager } from "@/shared/ui/Pager";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

const PAGE_SIZE = 50;
const GROUP_OPTIONS = ["", "platform", "identity", "account", "instrument"] as const;

export function PositionsPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const user = useCurrentUser();
  const overview = useOverview(portfolioId);
  const [params, setParams] = useSearchParams();
  const groupBy = readGroup(params.get("groupBy"));
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const instrumentId = params.get("instrumentId") ?? "";
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
  const query = useQuery({
    queryKey: [
      "positions",
      user.data?.id,
      portfolioId,
      generationId,
      groupBy,
      identityId,
      accountId,
      instrumentId,
      offset,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<PositionList> =>
      unwrapOrRedirect(
        await listPositions({
          path: { portfolioId },
          query: {
            groupBy: groupBy || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
            instrumentId: instrumentId || undefined,
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
        id: `${item.accountId}-${item.instrumentId}-${item.platformPositionRef}`,
        identity: item.identityName ?? item.identityId,
        account: item.accountName ?? item.accountId,
        platform: item.platformCode ?? "—",
        instrument: item.instrumentSymbol ?? item.instrumentId,
        ref: item.platformPositionRef,
        side: item.side,
        quantity: formatDecimal(item.quantity),
        mark: formatDecimal(item.selectedMarkPrice),
        long: formatDecimal(item.underlyingLongQuantity),
        short: formatDecimal(item.underlyingShortQuantity),
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
      <h2>Positions</h2>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Positions could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
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
          <span>Instrument id</span>
          <TextInput
            value={instrumentId}
            onChange={(event) => update({ instrumentId: event.target.value })}
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
              { key: "quantity", header: "Quantity", numeric: true },
              { key: "long", header: "Underlying long", numeric: true },
              { key: "short", header: "Underlying short", numeric: true },
              { key: "completeness", header: "Completeness" },
            ]}
            rows={query.data.groups.map((group) => ({
              key: group.key,
              label: group.label,
              quantity: formatDecimal(group.totals.quantity),
              long: formatDecimal(group.totals.underlyingLongQuantity),
              short: formatDecimal(group.totals.underlyingShortQuantity),
              completeness: group.completeness,
            }))}
          />
        </section>
      ) : null}
      <DataTable
        caption="Latest positions"
        columns={[
          { key: "identity", header: "Identity" },
          { key: "account", header: "Account" },
          { key: "platform", header: "Platform" },
          { key: "instrument", header: "Instrument" },
          { key: "ref", header: "Platform ref" },
          { key: "side", header: "Side" },
          { key: "quantity", header: "Quantity", numeric: true },
          { key: "mark", header: "Mark", numeric: true },
          { key: "long", header: "Underlying long", numeric: true },
          { key: "short", header: "Underlying short", numeric: true },
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
