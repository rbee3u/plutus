import type { UseQueryResult } from "@tanstack/react-query";
import type { OffsetPage } from "@/api";
import { Pager } from "@/shared/ui/Pager";
import type { ReactNode } from "react";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import { ActionButton } from "@/shared/ui/ActionButton";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import shared from "@/shared/styles/page.module.css";

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
    <div className={shared.headingRow}>
      {page ? (
        <Pager offset={page.offset} limit={page.limit} total={page.total} onChange={filters.page} />
      ) : null}
      {children}
      <ActionButton tone="ghost" disabled={!filters.hasFilters} onClick={filters.reset}>
        Clear filters
      </ActionButton>
    </div>
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
