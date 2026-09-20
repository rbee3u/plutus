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
        disabled={offset <= 0}
        onClick={() => onChange(Math.max(0, offset - limit))}
      >
        Previous
      </ActionButton>
      <span className={styles.muted}>
        {total === 0
          ? "0 rows"
          : `${String(offset + 1)}–${String(Math.min(next, total))} of ${String(total)}`}
      </span>
      <ActionButton type="button" disabled={next >= total} onClick={() => onChange(next)}>
        Next
      </ActionButton>
    </div>
  );
}
