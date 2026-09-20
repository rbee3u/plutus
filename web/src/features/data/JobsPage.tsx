import { type FormEvent, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createProjectionRebuild, getJob, getOverview, listJobs } from "@/api/generated/sdk.gen";
import type { Job } from "@/api/generated/types.gen";
import { safeJson } from "@/features/data/json";
import { useProjectionFollow } from "@/features/data/useProjectionFollow";
import { problemMessage, refreshCsrf } from "@/api/http";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { CursorPager } from "@/shared/ui/Pager";
import { TextArea } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export function JobsPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const follow = useProjectionFollow(portfolioId);
  const rebuildKey = useRef(crypto.randomUUID());
  const [rebuildOpen, setRebuildOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [detail, setDetail] = useState<Job | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };

  async function enqueueRebuild(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const overview = await queryClient.fetchQuery({
        queryKey: ["overview", user.data?.id, portfolioId],
        queryFn: async () =>
          unwrapOrRedirect(await getOverview({ path: { portfolioId } }), redirect),
      });
      await refreshCsrf();
      const result = await createProjectionRebuild({
        path: { portfolioId },
        headers: { "Idempotency-Key": rebuildKey.current },
        body: reason.trim() ? { reason: reason.trim() } : {},
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The projection rebuild could not be queued."));
        return;
      }
      rebuildKey.current = crypto.randomUUID();
      follow.follow(result.data, overview.context.generationId);
      setRebuildOpen(false);
      setReason("");
      await queryClient.invalidateQueries({ queryKey: ["jobs", user.data?.id, portfolioId] });
    } catch (cause) {
      setError(problemMessage(cause, "The projection rebuild could not be queued."));
    } finally {
      setBusy(false);
    }
  }

  async function openJob(jobId: string) {
    setError(null);
    const result = await getJob({ path: { portfolioId, jobId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    setDetail(unwrapOrRedirect(result, redirect));
  }

  return (
    <section className={styles.wide}>
      <h2>Jobs</h2>
      <p className={styles.lead}>
        Collection and projection work is durable. A rebuild from this screen is always a full
        projection rebuild.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {follow.notice ? <p className={styles.notice}>{follow.notice}</p> : null}
      {follow.job ? (
        <p className={styles.muted}>
          Watched job {follow.job.id}: {follow.job.status}
          {follow.job.lastErrorCode ? ` · ${follow.job.lastErrorCode}` : ""}
        </p>
      ) : null}
      <div className={styles.actions}>
        <ActionButton
          type="button"
          tone="primary"
          disabled={busy}
          onClick={() => setRebuildOpen(true)}
        >
          Rebuild projections
        </ActionButton>
      </div>
      <JobResults portfolioId={portfolioId} busy={busy} onOpen={(id) => void openJob(id)} />
      <Dialog
        open={rebuildOpen}
        title="Full projection rebuild"
        onClose={() => setRebuildOpen(false)}
      >
        <form className={styles.form} onSubmit={(event) => void enqueueRebuild(event)}>
          <p className={styles.lead}>
            This enqueues a full rebuild for the current Portfolio. Active equivalent jobs are
            merged instead of duplicated.
          </p>
          <label className={styles.field}>
            <span>Reason (optional)</span>
            <TextArea
              value={reason}
              maxLength={1024}
              onChange={(event) => setReason(event.target.value)}
              rows={3}
            />
          </label>
          <div className={styles.actions}>
            <ActionButton type="submit" tone="primary" disabled={busy}>
              Enqueue rebuild
            </ActionButton>
            <ActionButton type="button" onClick={() => setRebuildOpen(false)}>
              Cancel
            </ActionButton>
          </div>
        </form>
      </Dialog>
      <Dialog open={Boolean(detail)} title="Job diagnostics" onClose={() => setDetail(null)}>
        {detail ? (
          <>
            <dl className={styles.metaList}>
              <div>
                <dt>Kind</dt>
                <dd>{detail.kind}</dd>
              </div>
              <div>
                <dt>Status</dt>
                <dd>{detail.status}</dd>
              </div>
              <div>
                <dt>Attempts</dt>
                <dd>
                  {String(detail.attemptCount)} / {String(detail.maxAttempts)}
                </dd>
              </div>
              <div>
                <dt>Created</dt>
                <dd>{formatInstant(detail.createdAt)}</dd>
              </div>
            </dl>
            {detail.lastErrorCode || detail.lastErrorDetail ? (
              <p className={styles.alert} role="status">
                {detail.lastErrorCode}: {detail.lastErrorDetail}
              </p>
            ) : (
              <p className={styles.muted}>No sanitized error is recorded for this job.</p>
            )}
            <pre className={styles.jsonBlock}>{safeJson(detail.attempts ?? [])}</pre>
            <div className={styles.actions}>
              <ActionButton type="button" onClick={() => setDetail(null)}>
                Close
              </ActionButton>
            </div>
          </>
        ) : null}
      </Dialog>
    </section>
  );
}

function JobResults({
  portfolioId,
  busy,
  onOpen,
}: {
  portfolioId: string;
  busy: boolean;
  onOpen: (id: string) => void;
}) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const [cursors, setCursors] = useState<string[]>([""]);
  const cursor = cursors[cursors.length - 1] ?? "";
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const query = useQuery({
    queryKey: ["jobs", user.data?.id, portfolioId, cursor],
    enabled: Boolean(user.data?.id && portfolioId),
    refetchInterval: 4000,
    queryFn: async () =>
      unwrapOrRedirect(
        await listJobs({
          path: { portfolioId },
          query: { limit: PAGE_SIZE, cursor: cursor || undefined },
        }),
        redirect,
      ),
  });

  return (
    <>
      {query.error ? (
        <p className={styles.alert} role="alert">
          {query.error instanceof Error ? query.error.message : "Jobs could not be loaded."}
        </p>
      ) : null}
      <DataTable
        caption="Durable jobs"
        columns={[
          { key: "kind", header: "Kind" },
          { key: "status", header: "Status" },
          { key: "attempts", header: "Attempts" },
          { key: "error", header: "Error" },
          { key: "created", header: "Created" },
          { key: "finished", header: "Finished" },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          kind: item.kind,
          status: item.status,
          attempts: `${String(item.attemptCount)}/${String(item.maxAttempts)}`,
          error: item.lastErrorCode ?? "—",
          created: formatInstant(item.createdAt),
          finished: formatInstant(item.finishedAt),
        }))}
      />
      <ul className={styles.flowActions}>
        {(query.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>
              {item.kind} {item.status}
            </span>
            <span className={styles.actions}>
              <ActionButton type="button" disabled={busy} onClick={() => onOpen(item.id)}>
                Diagnostics
              </ActionButton>
            </span>
          </li>
        ))}
      </ul>
      <CursorPager
        canPrev={cursors.length > 1}
        hasMore={query.data?.page.hasMore ?? false}
        onPrev={() => setCursors((items) => items.slice(0, -1))}
        onNext={() => {
          const next = query.data?.page.nextCursor;
          if (next) setCursors((items) => [...items, next]);
        }}
      />
    </>
  );
}
