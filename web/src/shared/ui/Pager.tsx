import { formatCount } from "@/shared/lib/amount";
import { ActionButton } from "@/shared/ui/ActionButton";
import styles from "@/features/pages.module.css";

export function Pager({
  offset,
  limit,
  total,
  onChange,
}: {
  offset: number;
  limit: number;
  total: number;
  onChange: (offset: number) => void;
}) {
  const next = offset + limit;
  return (
    <div className={styles.actions}>
      <ActionButton
        type="button"
        tone="ghost"
        disabled={offset <= 0}
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
        disabled={next >= total}
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
