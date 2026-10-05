import type { ReactNode } from "react";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import { ActionButton } from "@/shared/ui/ActionButton";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import shared from "@/shared/styles/page.module.css";

export function HoldingsFilterRecovery({ filters }: { filters: HoldingsFilters }) {
  return (
    <QueryRecovery
      error={filters.identities.error}
      hasData={Boolean(filters.identities.data)}
      busy={filters.identities.isFetching}
      fallback="Filter candidates could not be loaded."
      onRetry={() => {
        void filters.identities.refetch();
      }}
    />
  );
}

export function HoldingsFilterFooter({
  filters,
  children,
}: {
  filters: HoldingsFilters;
  children?: ReactNode;
}) {
  return (
    <div className={shared.headingRow}>
      {children}
      <ActionButton tone="ghost" disabled={!filters.hasFilters} onClick={filters.reset}>
        Clear filters
      </ActionButton>
    </div>
  );
}
