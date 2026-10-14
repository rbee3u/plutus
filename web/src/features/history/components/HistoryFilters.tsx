import { TimeWindowFilters, type TimeWindowState } from "@/shared/ui/TimeWindowFilters";
import { type useHistoryPager } from "../hooks/useHistoryWindow";
import { HISTORY_PAGE_SIZES } from "@/shared/lib/timeWindow";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { Select } from "@/shared/ui/FormControls";
import { Pager } from "@/shared/ui/Pager";

export function HistoryFilters({ state }: { state: TimeWindowState }) {
  return <TimeWindowFilters state={state} ariaLabel="History time range" />;
}

export function HistoryPager({
  pager,
  total,
  label,
  busy = false,
}: {
  pager: ReturnType<typeof useHistoryPager>;
  total: number;
  label: string;
  busy?: boolean;
}) {
  return (
    <div className={shared.headingRow} aria-label={`${label} pagination`}>
      <label className={styles.periodSwitcher}>
        <span className={styles.periodLabel}>Rows per page</span>
        <Select
          className={styles.periodSelect}
          aria-label={`${label} rows per page`}
          value={pager.limit}
          disabled={busy}
          onChange={(event) => pager.onLimitChange(Number(event.target.value))}
        >
          {HISTORY_PAGE_SIZES.map((size) => (
            <option key={size} value={size}>
              {size}
            </option>
          ))}
        </Select>
      </label>
      <Pager
        offset={pager.offset}
        limit={pager.limit}
        total={total}
        onChange={pager.onChange}
        busy={busy}
      />
    </div>
  );
}
