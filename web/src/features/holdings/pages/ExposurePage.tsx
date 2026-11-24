import { useId, useState } from "react";
import { Link, useParams } from "react-router";
import { AlertTriangle } from "lucide-react";
import { keys, listExposure } from "@/api";
import type { ExposureCoverage, ExposureList } from "@/api";
import { MOBILE_LAYOUT_QUERY } from "@/shared/lib/useMobileScrollLock";
import { useMediaQuery } from "@/shared/lib/useMediaQuery";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { iconSize } from "@/shared/icons/iconSizes";
import { ExposureCards } from "../components/ExposureCards";
import { ExposureContributions } from "../components/ExposureContributions";
import { ExposureFilters } from "../components/ExposureFilters";
import { ExposureSummaryBar } from "../components/ExposureSummaryBar";
import { ExposureTable } from "../components/ExposureTable";
import { HoldingsListRecovery, HoldingsNoPoint } from "../components/HoldingsFilterControls";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";
import { useHoldingsFilters } from "../hooks/useHoldingsFilters";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

function CoverageNotice({ coverage }: { coverage: ExposureCoverage }) {
  const parts: string[] = [];
  if (coverage.missingAccountCount > 0) {
    parts.push(
      `${coverage.missingAccountCount} account${coverage.missingAccountCount === 1 ? "" : "s"} missing`,
    );
  }
  if (coverage.staleAccountCount > 0) {
    parts.push(
      `${coverage.staleAccountCount} stale account${coverage.staleAccountCount === 1 ? "" : "s"}`,
    );
  }
  if (coverage.excludedAssetCount > 0) {
    parts.push(
      `${coverage.excludedAssetCount} excluded asset${coverage.excludedAssetCount === 1 ? "" : "s"} omitted`,
    );
  }
  if (coverage.unattributedPositionCount > 0) {
    parts.push(
      `${coverage.unattributedPositionCount} unattributed position${coverage.unattributedPositionCount === 1 ? "" : "s"}`,
    );
  }
  if (parts.length === 0) return null;
  const omits = coverage.missingAccountCount > 0 || coverage.unattributedPositionCount > 0;
  const hasIssues =
    coverage.missingAccountCount > 0 ||
    coverage.staleAccountCount > 0 ||
    coverage.unattributedPositionCount > 0;
  return (
    <div className={`${shared.notice} ${styles.exposureNotice}`} role="status">
      <AlertTriangle size={iconSize.sm} aria-hidden="true" />
      <span>{parts.join(" · ")}</span>
      {omits ? <span className={shared.muted}>Known totals may omit their holdings.</span> : null}
      {hasIssues ? <a href="#exposure-issues">View issues</a> : null}
    </div>
  );
}

export function ExposurePage() {
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = usePortfolioQuery<ExposureList>({
    key: (userId, portfolioId) =>
      keys.exposure(userId, portfolioId, platform, identityId, symbol, offset, filters.readId),
    ready: filters.ready,
    load: (portfolioId) =>
      filters.read((cycleId) =>
        listExposure({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            q: symbol || undefined,
            offset,
            limit: 50,
            cycleId,
          },
        }),
      ),
  });
  const data = query.data;
  const detailId = useId();
  const mobile = useMediaQuery(MOBILE_LAYOUT_QUERY);
  const scope = [platform, identityId, symbol, offset, data?.context.cycleId ?? ""].join("\u0000");
  const [openState, setOpenState] = useState<{ scope: string; key: string | null }>({
    scope: "",
    key: null,
  });
  const open = openState.scope === scope ? openState.key : null;
  const toggle = (key: string) => {
    setOpenState({ scope, key: open === key ? null : key });
  };

  const empty = query.isPending ? (
    <TableEmptyState title="Loading exposure..." />
  ) : query.isError && !data ? (
    <TableEmptyState
      title="Exposure could not be loaded"
      help="Check the notice above to retry the request."
    />
  ) : !data?.context.cycleId ? (
    <HoldingsNoPoint filters={filters} />
  ) : !data.coverage.complete && data.items.length === 0 ? (
    <TableEmptyState
      title="Exposure data is incomplete"
      help="Some accounts have no compatible successful report."
    />
  ) : filters.hasFilters ? (
    <TableEmptyState
      title="No exposure matches these filters"
      help="Try adjusting the platform, identity, or asset filter."
    />
  ) : (
    <TableEmptyState
      title="No exposure to display"
      help="Balances and open positions across selected accounts appear here."
      action={<Link to={`/p/${portfolioId}/holdings/balances`}>View balances</Link>}
    />
  );

  const rows = data?.items ?? [];
  return (
    <section className={shared.wide}>
      <HoldingsListRecovery
        query={query}
        filters={filters}
        fallback="Exposure could not be loaded."
      />
      {data?.context.cycleId ? (
        <ExposureSummaryBar
          context={data.context}
          summary={data.summary}
          filtered={filters.hasFilters}
        />
      ) : null}
      <ExposureFilters filters={filters} />
      {data ? <CoverageNotice coverage={data.coverage} /> : null}
      {mobile ? (
        rows.length > 0 ? (
          <ExposureCards
            rows={rows}
            openKey={open}
            detailId={detailId}
            onToggle={toggle}
            cycleId={data?.context.cycleId ?? ""}
            platform={platform || undefined}
            identityId={identityId || undefined}
            timezone={data?.context.timezone}
          />
        ) : (
          <div className={styles.exposureMobileEmpty}>{empty}</div>
        )
      ) : (
        <ExposureTable
          rows={rows}
          platforms={filters.platforms.data?.items}
          timezone={data?.context.timezone}
          openKey={open}
          detailId={detailId}
          onToggle={toggle}
          busy={query.isFetching}
          empty={empty}
          renderDetail={(row) => (
            <ExposureContributions
              symbol={row.canonicalAssetSymbol}
              cycleId={data?.context.cycleId ?? ""}
              platform={platform || undefined}
              identityId={identityId || undefined}
              timezone={data?.context.timezone}
            />
          )}
        />
      )}
      <TableFilterFooter
        page={data?.page}
        onPageChange={filters.page}
        hasFilters={filters.hasFilters}
        onReset={filters.reset}
      >
        <span className={shared.muted}>assets · sorted by reference value</span>
      </TableFilterFooter>
      {data?.context.cycleId ? (
        <div id="exposure-issues">
          <IssuesPanel
            issues={data.issues}
            timezone={data.context.timezone}
            resolveScope={filters.resolveIssueScope}
          />
        </div>
      ) : null}
    </section>
  );
}
