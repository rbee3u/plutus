import { ChevronDown, ChevronRight } from "lucide-react";
import type { ExposureRow } from "@/api";
import { formatDecimal, formatPrice, formatUsd } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { NumericText } from "@/shared/ui/NumericText";
import { ExposureBadges } from "./ExposureTable";
import { ExposureContributions } from "./ExposureContributions";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function ExposureCards({
  rows,
  openKey,
  detailId,
  onToggle,
  cycleId,
  platform,
  identityId,
  timezone,
}: {
  rows: ExposureRow[];
  openKey: string | null;
  detailId: string;
  onToggle: (key: string) => void;
  cycleId: string;
  platform?: string;
  identityId?: string;
  timezone?: string;
}) {
  return (
    <ul className={styles.exposureCards} aria-label="Portfolio exposure">
      {rows.map((row) => {
        const open = openKey === row.canonicalAssetSymbol;
        const rowDetailId = `${detailId}-${row.canonicalAssetSymbol}`;
        return (
          <li className={styles.exposureCard} key={row.canonicalAssetSymbol}>
            <button
              type="button"
              className={styles.exposureCardMain}
              aria-expanded={open}
              aria-label={`${open ? "Collapse" : "Expand"} ${row.canonicalAssetSymbol} contributions`}
              aria-controls={open ? rowDetailId : undefined}
              onClick={() => onToggle(row.canonicalAssetSymbol)}
            >
              <span className={styles.exposureCardHead}>
                <span className={styles.exposureCardAsset}>
                  {row.canonicalAssetSymbol}
                  <ExposureBadges row={row} showFreshness />
                </span>
                <span className={styles.exposureCardQty}>
                  {row.netQuantity == null ? (
                    <span className={shared.muted}>—</span>
                  ) : (
                    <NumericText>{formatDecimal(row.netQuantity)}</NumericText>
                  )}
                </span>
              </span>
              <span className={styles.exposureCardMeta}>
                <span>
                  {row.valueUsd == null ? (
                    <span className={shared.muted}>—</span>
                  ) : (
                    <NumericText>{formatUsd(row.valueUsd)}</NumericText>
                  )}
                </span>
                <span>
                  {row.accountCount} {row.accountCount === 1 ? "account" : "accounts"}
                  {row.unknownAccountCount > 0 ? ` · ${row.unknownAccountCount} unknown` : ""}
                </span>
              </span>
              <span className={styles.exposureCardMeta}>
                <span>
                  Price{" "}
                  {row.priceUsd == null ? (
                    <span className={shared.muted}>—</span>
                  ) : (
                    formatPrice(row.priceUsd)
                  )}
                </span>
                <span className={styles.exposureCardChevron}>
                  {open ? (
                    <ChevronDown size={iconSize.sm} aria-hidden="true" />
                  ) : (
                    <ChevronRight size={iconSize.sm} aria-hidden="true" />
                  )}
                </span>
              </span>
            </button>
            {open ? (
              <div id={rowDetailId} className={styles.exposureCardDetail}>
                <ExposureContributions
                  symbol={row.canonicalAssetSymbol}
                  cycleId={cycleId}
                  platform={platform}
                  identityId={identityId}
                  timezone={timezone}
                  compact
                />
              </div>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}
