import type { UseQueryResult } from "@tanstack/react-query";
import type { OffsetPage } from "@/api";
import type { ReactNode } from "react";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";

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
