import type { UseQueryResult } from "@tanstack/react-query";
import type { OffsetPage } from "@/api";
import type { ReactNode } from "react";
import { Link, useParams } from "react-router";
import { identitySetupGuidance } from "@/shared/lib/useIdentities";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";

export function HoldingsNoPoint({ filters }: { filters: HoldingsFilters }) {
  const { portfolioId = "" } = useParams();
  const identities = filters.identities;
  if (!identities.isSuccess) {
    return (
      <TableEmptyState
        title="No valuation point yet"
        help={
          identities.isPending
            ? "Loading identities."
            : "Check the notice above to retry loading identities."
        }
      />
    );
  }
  const setup = identitySetupGuidance(identities.data.items);
  return (
    <TableEmptyState
      title={setup?.title ?? "No valuation point yet"}
      help={setup?.help ?? "Run a collection cycle to create the first valuation point."}
      action={
        setup ? (
          <Link to={`/p/${portfolioId}/data/identities`}>Manage identities</Link>
        ) : (
          <Link to={`/p/${portfolioId}/overview`}>Overview</Link>
        )
      }
    />
  );
}

export function HoldingsFilterRecovery({ filters }: { filters: HoldingsFilters }) {
  return (
    <QueryRecovery query={filters.identities} fallback="Filter candidates could not be loaded." />
  );
}

export function HoldingsFilterFooter({
  filters,
  children,
  page,
}: {
  filters: HoldingsFilters;
  children?: ReactNode;
  page?: OffsetPage;
}) {
  return (
    <TableFilterFooter
      page={page}
      onPageChange={filters.page}
      hasFilters={filters.hasFilters}
      onReset={filters.reset}
    >
      {children}
    </TableFilterFooter>
  );
}

export function HoldingsListRecovery({
  query,
  filters,
  fallback,
}: {
  query: Pick<UseQueryResult<unknown, unknown>, "error" | "data" | "isFetching" | "refetch">;
  filters: HoldingsFilters;
  fallback: string;
}) {
  return (
    <>
      <QueryRecovery query={query} fallback={fallback} />
      <HoldingsFilterRecovery filters={filters} />
    </>
  );
}
