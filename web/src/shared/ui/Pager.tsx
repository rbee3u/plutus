import { useEffect, useRef } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { formatCount } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import pagerStyles from "@/shared/ui/Pager.module.css";
import styles from "@/shared/styles/page.module.css";

export function Pager({
  offset,
  limit,
  total,
  onChange,
  busy = false,
}: {
  offset: number;
  limit: number;
  total: number;
  onChange: (offset: number) => void;
  busy?: boolean;
}) {
  const next = offset + limit;
  return (
    <div className={styles.actions}>
      <ActionButton
        type="button"
        tone="ghost"
        disabled={busy || offset <= 0}
        onClick={() => onChange(Math.max(0, offset - limit))}
      >
        Previous
      </ActionButton>
      <span className={styles.muted}>
        {total === 0
          ? "0 rows"
          : `${formatCount(offset + 1)}–${formatCount(Math.min(next, total))} of ${formatCount(total)}`}
      </span>
      <ActionButton
        type="button"
        tone="ghost"
        disabled={busy || next >= total}
        onClick={() => onChange(next)}
      >
        Next
      </ActionButton>
    </div>
  );
}

export function CursorPager({
  hasMore,
  canPrev,
  onPrev,
  onNext,
}: {
  hasMore: boolean;
  canPrev: boolean;
  onPrev: () => void;
  onNext: () => void;
}) {
  if (!canPrev && !hasMore) {
    return null;
  }
  return (
    <div className={styles.actions}>
      <ActionButton type="button" tone="ghost" disabled={!canPrev} onClick={onPrev}>
        Previous
      </ActionButton>
      <ActionButton type="button" tone="ghost" disabled={!hasMore} onClick={onNext}>
        Next
      </ActionButton>
    </div>
  );
}

/** A page number, or a visual gap that is not interactive. */
export type PageWindowItem = number | "ellipsis";

/**
 * The visible page numbers: first and last stay put, the current page keeps
 * two neighbours, and a gap of one page is filled rather than elided.
 */
export function pageWindow(totalPages: number, currentPage: number): PageWindowItem[] {
  if (totalPages <= 0) {
    return [];
  }
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  const current = Math.min(Math.max(currentPage, 1), totalPages);
  const pages = new Set([1, totalPages]);
  for (let page = current - 2; page <= current + 2; page += 1) {
    if (page >= 1 && page <= totalPages) {
      pages.add(page);
    }
  }
  const sorted = [...pages].sort((a, b) => a - b);
  const items: PageWindowItem[] = [];
  sorted.forEach((page, index) => {
    if (index > 0) {
      const gap = page - sorted[index - 1];
      if (gap === 2) {
        items.push(sorted[index - 1] + 1);
      } else if (gap > 2) {
        items.push("ellipsis");
      }
    }
    items.push(page);
  });
  return items;
}

/**
 * Numbered pager for server-paged history tables. The current page stays
 * focusable and marked; busy pages ignore clicks without losing focus.
 */
export function NumberedPager({
  page,
  totalPages,
  onChange,
  busy = false,
  label,
}: {
  page: number;
  totalPages: number;
  onChange: (page: number) => void;
  busy?: boolean;
  label?: string;
}) {
  const current = totalPages <= 0 ? 1 : Math.min(Math.max(page, 1), totalPages);
  const items = pageWindow(totalPages, current);
  const navRef = useRef<HTMLElement>(null);
  const focusPage = useRef<number | null>(null);

  /*
   * The URL change re-renders the page list, so the browser drops focus from
   * the clicked button. Restore it to the new current page without scrolling.
   */
  useEffect(() => {
    const target = focusPage.current;
    if (target === null) {
      return;
    }
    const active = document.activeElement;
    const focusWasInPager = active === document.body || navRef.current?.contains(active);
    if (focusWasInPager) {
      navRef.current
        ?.querySelector<HTMLButtonElement>(`button[aria-label="Page ${target}"]`)
        ?.focus({ preventScroll: true });
    }
    if (!busy) {
      focusPage.current = null;
    }
  }, [page, totalPages, busy]);

  return (
    <nav
      ref={navRef}
      className={pagerStyles.pager}
      aria-label={label ? `${label} pages` : "Pagination"}
    >
      <button
        type="button"
        className={pagerStyles.arrow}
        aria-label="Previous page"
        disabled={current <= 1 || totalPages === 0}
        aria-disabled={busy || undefined}
        onClick={() => {
          if (!busy) {
            focusPage.current = current - 1;
            onChange(current - 1);
          }
        }}
      >
        <ChevronLeft size={iconSize.sm} aria-hidden="true" />
      </button>
      {items.map((item, index) =>
        item === "ellipsis" ? (
          <span key={`ellipsis-${index}`} className={pagerStyles.ellipsis} aria-hidden="true">
            …
          </span>
        ) : (
          <button
            key={item}
            type="button"
            className={`${pagerStyles.page} ${item === current ? pagerStyles.current : ""}`}
            aria-label={`Page ${item}`}
            aria-current={item === current ? "page" : undefined}
            aria-disabled={busy || undefined}
            onClick={() => {
              if (!busy && item !== current) {
                focusPage.current = item;
                onChange(item);
              }
            }}
          >
            {item}
          </button>
        ),
      )}
      <button
        type="button"
        className={pagerStyles.arrow}
        aria-label="Next page"
        disabled={current >= totalPages || totalPages === 0}
        aria-disabled={busy || undefined}
        onClick={() => {
          if (!busy) {
            focusPage.current = current + 1;
            onChange(current + 1);
          }
        }}
      >
        <ChevronRight size={iconSize.sm} aria-hidden="true" />
      </button>
    </nav>
  );
}
