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
import { Codes } from "@/shared/ui/IssueList";
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
      <form
        className={styles.compactFilterBar}
        style={{ marginBottom: "var(--space-md)" }}
        onSubmit={(event) => event.preventDefault()}
      >
        <label className={styles.compactFilterField}>
          <span className={styles.compactFilterLabel}>Kind</span>
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
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          createdAt: (
            <span className={styles.cellStack}>
              {formatInstant(item.createdAt, timezone)}
              <span className={styles.cellMeta}>#{shortId(item.id)}</span>
            </span>
          ),
          kind: JOB_KIND_LABELS[item.kind],
          status: (
            <span className={styles.cellStack}>
              <span>
                <StatusBadge value={item.status} />
              </span>
              {item.lastErrorClass || item.lastDetail ? (
                <span className={styles.cellStack} style={{ marginTop: "2px" }}>
                  {item.lastErrorClass ? <Codes values={[item.lastErrorClass]} /> : null}
                  {item.lastDetail ? (
                    <span className={styles.cellMeta} style={{ color: "var(--danger)" }}>
                      {item.lastDetail}
                    </span>
                  ) : null}
                </span>
              ) : null}
            </span>
          ),
          trigger: capitalize(item.trigger),
          attempts: `${formatCount(item.attemptCount)}/${formatCount(item.maxAttempts)}`,
          startedAt: formatInstant(item.startedAt, timezone),
          finishedAt: formatInstant(item.finishedAt, timezone),
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
