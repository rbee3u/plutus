import { formatCount } from "@/shared/lib/amount";
import { useQuery } from "@tanstack/react-query";
import { listJobs } from "@/api/generated/sdk.gen";
import type { JobKind } from "@/api/generated/types.gen";
import { useCursorPages } from "@/features/data/activity/useCursorPages";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { capitalize } from "@/shared/lib/text";
import { StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Select } from "@/shared/ui/FormControls";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export const JOB_KINDS: readonly JobKind[] = ["collection_cycle", "valuation_recalc"];
const JOB_KIND_LABELS: Record<JobKind, string> = {
  collection_cycle: "Collection Cycle",
  valuation_recalc: "Valuation recalculation",
};

export function JobsPanel({
  portfolioId,
  kind,
  timezone,
  onKind,
}: {
  portfolioId: string;
  kind: JobKind | "";
  timezone?: string;
  onKind: (kind: string) => void;
}) {
  return (
    <>
      <p className={styles.lead}>
        Background work for this Portfolio: Collection Cycles and valuation recalculations, with
        their attempts and outcomes.
      </p>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>Kind</span>
          <Select value={kind} onChange={(event) => onKind(event.target.value)}>
            <option value="">All kinds</option>
            {JOB_KINDS.map((item) => (
              <option key={item} value={item}>
                {JOB_KIND_LABELS[item]}
              </option>
            ))}
          </Select>
        </label>
      </form>
      <JobsTable key={kind} portfolioId={portfolioId} kind={kind} timezone={timezone} />
    </>
  );
}

function JobsTable({
  portfolioId,
  kind,
  timezone,
}: {
  portfolioId: string;
  kind: JobKind | "";
  timezone?: string;
}) {
  const { userId, redirect } = useSessionScope();
  const pages = useCursorPages();
  const query = useQuery({
    queryKey: ["jobs", userId, portfolioId, kind, pages.cursor],
    enabled: Boolean(userId && portfolioId),
    refetchInterval: 5000,
    queryFn: async () =>
      unwrapOrRedirect(
        await listJobs({
          path: { portfolioId },
          query: { kind: kind || undefined, limit: PAGE_SIZE, cursor: pages.cursor || undefined },
        }),
        redirect,
      ),
  });

  return (
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Jobs could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <DataTable
        caption="Jobs, newest first"
        columns={[
          { key: "createdAt", header: "Created", nowrap: true },
          { key: "kind", header: "Kind", nowrap: true },
          { key: "status", header: "Status" },
          { key: "trigger", header: "Trigger" },
          { key: "attempts", header: "Attempts", numeric: true },
          { key: "startedAt", header: "Started", nowrap: true },
          { key: "finishedAt", header: "Finished", nowrap: true },
          { key: "outcome", header: "Outcome" },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          createdAt: (
            <span className={styles.cellStack}>
              {formatInstant(item.createdAt, timezone)}
              <span className={styles.cellMeta}>{shortId(item.id)}</span>
            </span>
          ),
          kind: JOB_KIND_LABELS[item.kind],
          status: <StatusBadge value={item.status} />,
          trigger: capitalize(item.trigger),
          attempts: `${formatCount(item.attemptCount)}/${formatCount(item.maxAttempts)}`,
          startedAt: formatInstant(item.startedAt, timezone),
          finishedAt: formatInstant(item.finishedAt, timezone),
          outcome:
            item.lastErrorClass || item.lastDetail ? (
              <span className={styles.cellStack}>
                {item.lastErrorClass ? <span>{item.lastErrorClass}</span> : null}
                {item.lastDetail ? (
                  <span className={styles.cellMeta}>{item.lastDetail}</span>
                ) : null}
              </span>
            ) : (
              "—"
            ),
        }))}
        empty={
          query.isPending
            ? "Loading jobs."
            : kind
              ? `No ${JOB_KIND_LABELS[kind].toLowerCase()} job yet.`
              : "No jobs yet. Jobs are created by the collection schedule, “Collect now”, and valuation recalculations."
        }
      />
      <CursorPager
        canPrev={pages.canPrev}
        hasMore={query.data?.page.hasMore ?? false}
        onPrev={pages.prev}
        onNext={() => pages.next(query.data?.page.nextCursor)}
      />
    </>
  );
}
