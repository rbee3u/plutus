import { formatCount } from "@/shared/lib/amount";
import { keys, listJobs } from "@/api";
import type { JobKind, JobList } from "@/api";
import { useCursorPages } from "@/features/manage/activity/useCursorPages";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { capitalize } from "@/shared/lib/text";
import { StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Select } from "@/shared/ui/FormControls";
import { Codes } from "@/shared/ui/IssueList";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

const PAGE_SIZE = 50;

export const JOB_KINDS: readonly JobKind[] = ["collection_cycle", "valuation_recalc"];
const JOB_KIND_LABELS: Record<JobKind, string> = {
  collection_cycle: "Collection Cycle",
  valuation_recalc: "Valuation recalculation",
};

export function JobsPanel({
  kind,
  timezone,
  onKind,
}: {
  kind: JobKind | "";
  timezone?: string;
  onKind: (kind: string) => void;
}) {
  return (
    <>
      <form
        className={shared.compactFilterBar}
        style={{ marginBottom: "var(--space-md)" }}
        onSubmit={(event) => event.preventDefault()}
      >
        <label className={shared.compactFilterField}>
          <span className={shared.compactFilterLabel}>Kind</span>
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
      <JobsTable key={kind} kind={kind} timezone={timezone} />
    </>
  );
}

function JobsTable({ kind, timezone }: { kind: JobKind | ""; timezone?: string }) {
  const pages = useCursorPages();
  const query = usePortfolioQuery<JobList>({
    key: (userId, portfolioId) => keys.jobs(userId, portfolioId, kind, pages.cursor),
    ready: true,
    load: (portfolioId) =>
      listJobs({
        path: { portfolioId },
        query: { kind: kind || undefined, limit: PAGE_SIZE, cursor: pages.cursor || undefined },
      }),
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
            <span className={shared.cellStack}>
              {formatInstant(item.createdAt, timezone)}
              <span className={shared.cellMeta}>#{shortId(item.id)}</span>
            </span>
          ),
          kind: JOB_KIND_LABELS[item.kind],
          status: (
            <span className={shared.cellStack}>
              <span>
                <StatusBadge value={item.status} />
              </span>
              {item.lastErrorClass || item.lastDetail ? (
                <span className={shared.cellStack} style={{ marginTop: "2px" }}>
                  {item.lastErrorClass ? <Codes values={[item.lastErrorClass]} /> : null}
                  {item.lastDetail ? (
                    <span className={shared.cellMeta} style={{ color: "var(--danger)" }}>
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
