import { formatCount } from "@/shared/lib/amount";
import { HISTORY_PAGE_SIZES } from "@/shared/lib/timeWindow";
import { Select } from "@/shared/ui/FormControls";
import { NumberedPager } from "@/shared/ui/Pager";
import { TimeWindowFilters, type TimeWindowState } from "@/shared/ui/TimeWindowFilters";
import { type useHistoryPager } from "../hooks/useHistoryWindow";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";

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
  const totalPages = Math.ceil(total / pager.limit);
  const offset = totalPages === 0 ? 0 : Math.min(pager.offset, (totalPages - 1) * pager.limit);
  const page = totalPages === 0 ? 1 : Math.floor(offset / pager.limit) + 1;
  const start = total === 0 ? 0 : offset + 1;
  const end = Math.min(offset + pager.limit, total);
  const range =
    total === 0
      ? busy
        ? "Loading…"
        : "0 rows"
      : `${formatCount(start)}–${formatCount(end)} of ${formatCount(total)}`;
  const announcement =
    total === 0
      ? busy
        ? `${label}: loading`
        : `${label}: 0 rows`
      : `${label}: page ${page} of ${totalPages}, rows ${formatCount(start)}–${formatCount(end)} of ${formatCount(total)}`;

  return (
    <div className={styles.tableToolbar} role="group" aria-label={`${label} pagination`}>
      <label className={styles.pagerMeta}>
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
      <span className={styles.rangeText}>{range}</span>
      <NumberedPager
        page={page}
        totalPages={totalPages}
        onChange={(next) => pager.onChange((next - 1) * pager.limit)}
        busy={busy}
        label={label}
      />
      <span className={shared.srOnly} role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </span>
    </div>
  );
}
