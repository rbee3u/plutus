import type { ReactNode } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { ExposureRow, Platform } from "@/api";
import {
  formatCount,
  formatDecimal,
  formatPrice,
  formatUsd,
  isZeroDecimal,
} from "@/shared/lib/amount";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { iconSize } from "@/shared/icons/iconSizes";
import { Badge } from "@/shared/ui/Badge";
import { DataTable, type DataColumn, type DataRow } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import { ObservedCell } from "./ObservedCell";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function ExposureBadges({
  row,
  showFreshness = false,
}: {
  row: ExposureRow;
  showFreshness?: boolean;
}) {
  return (
    <span className={styles.exposureBadges}>
      {row.state === "partial" ? <Badge tone="warn">Partial</Badge> : null}
      {row.state === "unknown" ? <Badge tone="warn">Unknown</Badge> : null}
      {row.state === "complete" && isZeroDecimal(row.netQuantity) ? (
        <Badge tone="neutral">Flat</Badge>
      ) : null}
      {showFreshness && row.freshness === "stale" ? <Badge tone="warn">Stale</Badge> : null}
    </span>
  );
}

function netCell(row: ExposureRow) {
  if (row.netQuantity == null) return <span className={shared.muted}>—</span>;
  return <NumericText>{formatDecimal(row.netQuantity)}</NumericText>;
}

function priceCell(row: ExposureRow, platforms?: Platform[]) {
  if (row.priceUsd == null) return <span className={shared.muted}>—</span>;
  const platform = platformLabel(platforms, row.priceSourcePlatform ?? "");
  const source = row.priceSource === "contract" ? "contract mark" : "spot";
  const symbol = row.priceSourceSymbol ?? row.canonicalAssetSymbol;
  return (
    <HoverDetails
      name={`${row.canonicalAssetSymbol} reference price`}
      label={<NumericText>{formatPrice(row.priceUsd)}</NumericText>}
    >
      <p>
        Reference price from {platform} {symbol} ({source}).
      </p>
      <p className={shared.muted}>Display-only; it is not a valuation or liquidation value.</p>
    </HoverDetails>
  );
}

function accountsCell(row: ExposureRow) {
  return (
    <span className={styles.exposureAccounts}>
      <NumericText>{formatCount(row.accountCount)}</NumericText>
      {row.unknownAccountCount > 0 ? (
        <span className={shared.cellMeta}>{row.unknownAccountCount} unknown</span>
      ) : null}
    </span>
  );
}

function contributionIssues(row: ExposureRow, timezone?: string) {
  return (
    <ObservedCell
      freshness={row.freshness}
      issues={row.issues}
      name={row.canonicalAssetSymbol}
      timezone={timezone}
    />
  );
}

function toggleButton(
  row: ExposureRow,
  open: boolean,
  detailId: string,
  onToggle: (key: string) => void,
) {
  return (
    <button
      type="button"
      className={styles.exposureToggle}
      aria-expanded={open}
      aria-controls={open ? detailId : undefined}
      aria-label={`${open ? "Collapse" : "Expand"} ${row.canonicalAssetSymbol} contributions`}
      onClick={(event) => {
        event.stopPropagation();
        onToggle(row.canonicalAssetSymbol);
      }}
    >
      {open ? (
        <ChevronDown size={iconSize.sm} aria-hidden="true" />
      ) : (
        <ChevronRight size={iconSize.sm} aria-hidden="true" />
      )}
    </button>
  );
}

const EXPOSURE_COLUMNS: DataColumn[] = [
  { key: "expand", header: "", width: "34px" },
  { key: "asset", header: "Asset", width: "190px" },
  { key: "net", header: "Net exposure", numeric: true, width: "170px" },
  { key: "price", header: "Price (USD)", numeric: true, width: "140px" },
  { key: "value", header: "Value (USD)", numeric: true, width: "150px" },
  { key: "accounts", header: "Accounts", numeric: true, width: "120px" },
  { key: "issues", header: "Issues", nowrap: true, width: "110px" },
];

export function ExposureTable({
  rows,
  platforms,
  timezone,
  openKey,
  detailId,
  onToggle,
  renderDetail,
  empty,
  busy,
}: {
  rows: ExposureRow[];
  platforms?: Platform[];
  timezone?: string;
  openKey: string | null;
  detailId: string;
  onToggle: (key: string) => void;
  renderDetail: (row: ExposureRow) => ReactNode;
  empty: ReactNode;
  busy?: boolean;
}) {
  return (
    <DataTable
      caption="Portfolio exposure by canonical asset"
      columns={EXPOSURE_COLUMNS}
      tableLayout="auto"
      busy={busy}
      rows={rows.map((row): DataRow => {
        const open = openKey === row.canonicalAssetSymbol;
        const rowDetailId = `${detailId}-${row.canonicalAssetSymbol}`;
        return {
          id: row.canonicalAssetSymbol,
          expand: toggleButton(row, open, rowDetailId, onToggle),
          asset: (
            <span className={styles.exposureAsset}>
              <span>{row.canonicalAssetSymbol}</span>
              <ExposureBadges row={row} />
            </span>
          ),
          net: netCell(row),
          price: priceCell(row, platforms),
          value:
            row.valueUsd == null ? (
              <span className={shared.muted}>—</span>
            ) : (
              formatUsd(row.valueUsd)
            ),
          accounts: accountsCell(row),
          issues: contributionIssues(row, timezone),
        };
      })}
      openKey={openKey ?? undefined}
      detailId={openKey ? `${detailId}-${openKey}` : undefined}
      renderDetail={(dataRow) => {
        const row = rows.find((item) => item.canonicalAssetSymbol === dataRow.id);
        return row ? renderDetail(row) : null;
      }}
      onRowClick={onToggle}
      rowClassName={(row) => (row.id === openKey ? shared.openRow : undefined)}
      empty={empty}
    />
  );
}
