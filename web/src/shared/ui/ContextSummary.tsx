import type { ReadContext } from "@/api/generated/types.gen";
import { formatInstant } from "@/shared/lib/datetime";
import styles from "@/features/pages.module.css";

/**
 * When the view was evaluated and in which timezone, as one quiet line. A
 * series view has no single point, so only the timezone shows.
 */
export function ContextSummary({ context }: { context: ReadContext }) {
  return (
    <p className={styles.muted}>
      {context.evaluatedAt
        ? `Latest Valuation Point ${formatInstant(context.evaluatedAt, context.timezone)} · ${context.timezone}`
        : `Times in ${context.timezone}`}
    </p>
  );
}
