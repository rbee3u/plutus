import type { OffsetPage } from "@/api";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Pager } from "@/shared/ui/Pager";
import type { ReactNode } from "react";
import shared from "@/shared/styles/page.module.css";

export function TableFilterFooter({
  page,
  onPageChange,
  hasFilters,
  onReset,
  children,
}: {
  page?: OffsetPage;
  onPageChange?: (offset: number) => void;
  hasFilters: boolean;
  onReset: () => void;
  children?: ReactNode;
}) {
  return (
    <div className={shared.headingRow}>
      {page && onPageChange ? (
        <Pager offset={page.offset} limit={page.limit} total={page.total} onChange={onPageChange} />
      ) : null}
      {children}
      <ActionButton tone="ghost" disabled={!hasFilters} onClick={onReset}>
        Clear filters
      </ActionButton>
    </div>
  );
}
