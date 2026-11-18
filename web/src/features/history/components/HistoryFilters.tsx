import { useLayoutEffect, useRef } from "react";
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

/** The element that scrolls this footer, or null when the document scrolls. */
function scrollerOf(element: Element): HTMLElement | null {
  let node = element.parentElement;
  while (node && node !== document.body && node !== document.documentElement) {
    const { overflowY } = getComputedStyle(node);
    if (overflowY === "auto" || overflowY === "scroll") {
      return node;
    }
    node = node.parentElement;
  }
  return null;
}

/**
 * The pager is its table's footer: it sits below the rows, and a page change
 * keeps it where the reader left it. The table's height changes when the rows
 * change, so the scroll absorbs the difference instead: the rows move, the
 * pager does not, and titles keep scrolling normally above it.
 */
export function HistoryPager({
  pager,
  total,
  label,
  busy = false,
  signature,
}: {
  pager: ReturnType<typeof useHistoryPager>;
  total: number;
  label: string;
  busy?: boolean;
  /** Changes whenever the rows on screen change, so the anchor knows when to settle. */
  signature: string;
}) {
  const rootRef = useRef<HTMLDivElement>(null);
  const anchor = useRef<number | null>(null);
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

  const rememberPlace = () => {
    anchor.current = rootRef.current?.getBoundingClientRect().top ?? null;
  };

  /*
   * Runs before paint once the rows are in the document: scroll by however much
   * the footer moved, so its place on screen is the one the reader paged from.
   * A read that ends without new rows (an error) just drops the anchor.
   */
  useLayoutEffect(() => {
    const root = rootRef.current;
    const top = anchor.current;
    if (!root || top === null) {
      return;
    }
    const moved = root.getBoundingClientRect().top - top;
    if (Math.abs(moved) >= 0.5) {
      const scroller = scrollerOf(root);
      if (scroller) {
        scroller.scrollTop += moved;
      } else {
        window.scrollBy(0, moved);
      }
      anchor.current = null;
    } else if (!busy) {
      anchor.current = null;
    }
  }, [signature, busy]);

  return (
    <div
      ref={rootRef}
      className={styles.tableFooter}
      role="group"
      aria-label={`${label} pagination`}
    >
      <label className={styles.pagerMeta}>
        <span className={styles.periodLabel}>Rows per page</span>
        <Select
          className={styles.periodSelect}
          aria-label={`${label} rows per page`}
          value={pager.limit}
          disabled={busy}
          onChange={(event) => {
            rememberPlace();
            pager.onLimitChange(Number(event.target.value));
          }}
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
        onChange={(next) => {
          rememberPlace();
          pager.onChange((next - 1) * pager.limit);
        }}
        busy={busy}
        label={label}
      />
      <span className={shared.srOnly} role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </span>
    </div>
  );
}
