import {
  listAccounts,
  listExposures,
  listIdentities,
  searchCatalogAssets,
} from "@/api/generated/sdk.gen";
import type { ExposureList } from "@/api/generated/types.gen";
import { chartNumber, formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ChartBlock } from "@/shared/ui/ChartBlock";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Pager } from "@/shared/ui/Pager";
import { Select } from "@/shared/ui/FormControls";
import { useOverview } from "@/features/overview/useOverview";
import styles from "@/features/pages.module.css";
import type { EChartsCoreOption } from "echarts/core";
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

const PAGE_SIZE = 50;
const GROUP_OPTIONS = ["", "asset", "platform", "identity", "account"] as const;

export function ExposurePage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const user = useCurrentUser();
  const overview = useOverview(portfolioId);
  const [params, setParams] = useSearchParams();
  const groupBy = readGroup(params.get("groupBy"));
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const assetId = params.get("assetId") ?? "";
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
      "exposures",
      user.data?.id,
      portfolioId,
      generationId,
      groupBy,
      identityId,
      accountId,
      assetId,
      offset,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async (): Promise<ExposureList> =>
      unwrapOrRedirect(
        await listExposures({
          path: { portfolioId },
          query: {
            groupBy: groupBy || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
            assetId: assetId || undefined,
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
        id: `${item.assetId}-${item.accountId ?? item.identityId ?? item.platformId ?? "asset"}`,
        asset: item.assetCode ?? item.assetId,
        identity: item.identityName ?? item.identityId ?? "—",
        account: item.accountName ?? item.accountId ?? "—",
        platform: item.platformCode ?? "—",
        long: formatDecimal(item.longQuantity),
        short: formatDecimal(item.shortQuantity),
        net: formatDecimal(item.netQuantity),
        longValue: formatDecimal(item.longValue),
        shortValue: formatDecimal(item.shortValue),
        netValue: formatDecimal(item.netValue),
        evaluated: formatInstant(item.evaluatedAt, query.data?.context.timezone),
        freshness: item.freshness,
        completeness: item.completeness,
      })),
    [query.data],
  );

  const chartOption = useMemo((): EChartsCoreOption | null => {
    const items = query.data?.items ?? [];
    if (items.length === 0) {
      return null;
    }
    return {
      legend: { data: ["Long", "Short", "Net"] },
      xAxis: {
        type: "category",
        data: items.map((item) =>
          [item.assetCode ?? item.assetId, item.identityName, item.accountName]
            .filter(Boolean)
            .join(" · "),
        ),
      },
      yAxis: { type: "value", name: "Quantity" },
      series: [
        { type: "bar", name: "Long", data: items.map((item) => chartNumber(item.longQuantity)) },
        {
          type: "bar",
          name: "Short",
          data: items.map((item) => chartNumber(item.shortQuantity)),
          itemStyle: { borderWidth: 1, borderType: "dashed" },
        },
        {
          type: "line",
          name: "Net",
          data: items.map((item) => chartNumber(item.netQuantity)),
          symbol: "diamond",
        },
      ],
    };
  }, [query.data]);

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
      <h2>Exposure</h2>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Exposure could not be loaded."
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
      </form>
      {query.data && groupBy ? (
        <section className={styles.panel}>
          <h3>Group totals</h3>
          <DataTable
            caption="Server-calculated exposure group totals"
            columns={[
              { key: "label", header: "Group" },
              { key: "long", header: "Long", numeric: true },
              { key: "short", header: "Short", numeric: true },
              { key: "net", header: "Net", numeric: true },
              { key: "longValue", header: "Long value", numeric: true },
              { key: "shortValue", header: "Short value", numeric: true },
              { key: "netValue", header: "Net value", numeric: true },
              { key: "completeness", header: "Completeness" },
            ]}
            rows={query.data.groups.map((group) => ({
              key: group.key,
              label: group.label,
              long: formatDecimal(group.totals.longQuantity),
              short: formatDecimal(group.totals.shortQuantity),
              net: formatDecimal(group.totals.netQuantity),
              longValue: formatDecimal(group.totals.longValue),
              shortValue: formatDecimal(group.totals.shortValue),
              netValue: formatDecimal(group.totals.netValue),
              completeness: group.completeness,
            }))}
          />
        </section>
      ) : null}
      <ChartBlock
        title="Exposure quantities"
        summary="Long and short stay visible even when net is zero. Values come from the active generation; this page does not sum paginated rows."
        option={chartOption}
        empty="No exposure rows are available yet."
        columns={[
          { key: "asset", header: "Asset" },
          { key: "identity", header: "Identity" },
          { key: "account", header: "Account" },
          { key: "platform", header: "Platform" },
          { key: "long", header: "Long", numeric: true },
          { key: "short", header: "Short", numeric: true },
          { key: "net", header: "Net", numeric: true },
          { key: "longValue", header: "Long value", numeric: true },
          { key: "shortValue", header: "Short value", numeric: true },
          { key: "netValue", header: "Net value", numeric: true },
          { key: "evaluated", header: "Evaluated" },
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
