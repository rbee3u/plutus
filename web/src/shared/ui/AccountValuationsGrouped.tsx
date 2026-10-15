import { useMemo, type ReactNode } from "react";
import type { AccountValuation, Platform } from "@/api";
import { formatUsd } from "@/shared/lib/amount";
import {
  ACCOUNT_VALUATION_COLUMNS,
  accountValuationRow,
  groupAccountValuations,
} from "@/shared/lib/valuationRows";
import { Badge } from "@/shared/ui/Badge";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "./AccountValuationsGrouped.module.css";
import shared from "@/shared/styles/page.module.css";

export function AccountValuationsGrouped({
  accounts,
  timezone,
  platforms,
  empty = "No accounts were valued.",
  renderAccountAction,
  renderDetail,
  openKey,
  onRowClick,
  detailId,
}: {
  accounts: AccountValuation[];
  timezone?: string;
  platforms?: Platform[];
  empty?: string;
  renderAccountAction?: (account: AccountValuation) => ReactNode;
  renderDetail?: (row: DataRow) => ReactNode;
  openKey?: string;
  onRowClick?: (key: string) => void;
  detailId?: string;
}) {
  const groups = useMemo(() => groupAccountValuations(accounts, platforms), [accounts, platforms]);

  if (groups.length === 0) {
    return <p className={shared.muted}>{empty}</p>;
  }

  return (
    <div className={styles.groupedList}>
      {groups.map((group) => (
        <section key={group.key} className={styles.identityGroupCard}>
          <header className={styles.identityGroupHeader}>
            <div className={styles.identityGroupTitle}>
              <span className={styles.identityGroupPlatform}>{group.platformLabel}</span>
              <span className={styles.identityGroupDivider}>·</span>
              <span>{group.identityName}</span>
            </div>
            <div className={styles.identityGroupMeta}>
              {group.issuesCount > 0 ? (
                <Badge tone="warn">
                  {group.issuesCount} {group.issuesCount === 1 ? "issue" : "issues"}
                </Badge>
              ) : null}
              {group.totalUsd ? (
                <span className={styles.identityGroupSubtotal}>
                  <NumericText>{formatUsd(group.totalUsd)}</NumericText>
                </span>
              ) : null}
            </div>
          </header>
          <DataTable
            caption={`Accounts for ${group.platformLabel} ${group.identityName}`}
            columns={ACCOUNT_VALUATION_COLUMNS}
            rows={group.accounts.map((item) => {
              const row = accountValuationRow(item, timezone);
              if (renderAccountAction) {
                row.account = renderAccountAction(item);
              }
              return row;
            })}
            renderDetail={renderDetail}
            openKey={openKey}
            onRowClick={onRowClick}
            detailId={detailId}
          />
        </section>
      ))}
    </div>
  );
}
