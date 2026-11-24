import { ArrowDownWideNarrow } from "lucide-react";
import { iconSize } from "@/shared/icons/iconSizes";
import { HoldingsFilterHeader } from "./HoldingsFilterHeader";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import styles from "../styles.module.css";

/** Portfolio-level controls stay outside the table so mobile cards can reuse them. */
export function ExposureFilters({ filters }: { filters: HoldingsFilters }) {
  return (
    <div className={styles.exposureFilters}>
      <div className={styles.exposureFilter}>
        <HoldingsFilterHeader label="Platform" filterKey="platform" filters={filters} />
      </div>
      <div className={styles.exposureFilter}>
        <HoldingsFilterHeader label="Identity" filterKey="identityId" filters={filters} />
      </div>
      <div className={`${styles.exposureFilter} ${styles.exposureFilterSearch}`}>
        <HoldingsFilterHeader label="Asset" filterKey="symbol" filters={filters} />
      </div>
      <span
        className={styles.exposureSort}
        title="Rows are sorted by absolute reference value; assets without a value follow."
      >
        <ArrowDownWideNarrow size={iconSize.xs} aria-hidden="true" />
        Largest first
      </span>
    </div>
  );
}
