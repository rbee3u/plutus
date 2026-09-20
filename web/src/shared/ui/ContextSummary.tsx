import type { ProjectionContext } from "@/api/generated/types.gen";
import { formatInstant } from "@/shared/lib/datetime";
import styles from "@/features/pages.module.css";

export function ContextSummary({ context }: { context: ProjectionContext }) {
  return (
    <dl className={styles.metaList}>
      <div>
        <dt>Display asset</dt>
        <dd>{context.displayAsset.code}</dd>
      </div>
      <div>
        <dt>Timezone</dt>
        <dd>{context.timezone}</dd>
      </div>
      <div>
        <dt>Cutoff</dt>
        <dd>{formatInstant(context.cutoffAt, context.timezone)}</dd>
      </div>
      <div>
        <dt>Completeness</dt>
        <dd>{context.completeness}</dd>
      </div>
      <div>
        <dt>Coverage</dt>
        <dd>{context.coverage}</dd>
      </div>
      <div>
        <dt>Freshness</dt>
        <dd>{context.freshness}</dd>
      </div>
      {context.rebuildPending ? (
        <div>
          <dt>Rebuild</dt>
          <dd>A newer timezone is queued; this view still uses the active generation.</dd>
        </div>
      ) : null}
    </dl>
  );
}
