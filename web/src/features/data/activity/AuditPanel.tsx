import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { listAuditEvents } from "@/api/generated/sdk.gen";
import type { AuditEvent } from "@/api/generated/types.gen";
import { useCursorPages } from "@/features/data/activity/useCursorPages";
import { safeJson } from "@/features/data/json";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { unwrapOrRedirect, useCurrentUser, useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export function AuditPanel({
  portfolioId,
  action,
  resourceType,
  timezone,
  onFilter,
}: {
  portfolioId: string;
  action: string;
  resourceType: string;
  timezone?: string;
  onFilter: (next: Record<string, string>) => void;
}) {
  return (
    <>
      <form className={styles.filterBar} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field} style={{ flex: "1 1 200px" }}>
          <span>Action</span>
          <TextInput
            value={action}
            onChange={(event) => onFilter({ action: event.target.value })}
            placeholder="e.g. identity.enable"
          />
        </label>
        <label className={styles.field} style={{ flex: "1 1 200px" }}>
          <span>Resource type</span>
          <TextInput
            value={resourceType}
            onChange={(event) => onFilter({ resourceType: event.target.value })}
            placeholder="e.g. identity"
          />
        </label>
      </form>
      <AuditResults
        key={`${action}|${resourceType}`}
        portfolioId={portfolioId}
        action={action}
        resourceType={resourceType}
        timezone={timezone}
      />
    </>
  );
}

function AuditResults({
  portfolioId,
  action,
  resourceType,
  timezone,
}: {
  portfolioId: string;
  action: string;
  resourceType: string;
  timezone?: string;
}) {
  const { userId, redirect } = useSessionScope();
  const me = useCurrentUser();
  const pages = useCursorPages();
  const [detail, setDetail] = useState<AuditEvent | null>(null);
  const query = useQuery({
    queryKey: ["audit-events", userId, portfolioId, action, resourceType, pages.cursor],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAuditEvents({
          path: { portfolioId },
          query: {
            action: action || undefined,
            resourceType: resourceType || undefined,
            limit: PAGE_SIZE,
            cursor: pages.cursor || undefined,
          },
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
        fallback="Audit events could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <DataTable
        caption="Portfolio audit events"
        onRowClick={(key) => {
          const item = (query.data?.items ?? []).find((i) => i.id === key);
          if (item) setDetail(item);
        }}
        columns={[
          { key: "occurred", header: "Occurred", nowrap: true },
          { key: "action", header: "Action" },
          { key: "resource", header: "Resource" },
          { key: "actor", header: "Actor" },
          { key: "actions", header: "", nowrap: true },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          occurred: formatInstant(item.occurredAt, timezone),
          action: item.action,
          resource: (
            <span className={styles.cellStack}>
              <span>{item.resourceType}</span>
              {item.resourceId ? (
                <span className={styles.cellMeta}>{shortId(item.resourceId)}</span>
              ) : null}
            </span>
          ),
          actor: !item.actorUserId
            ? "system"
            : item.actorUserId === me.data?.id
              ? me.data.username
              : shortId(item.actorUserId),
          actions: (
            <span className={styles.rowActions}>
              <ActionButton type="button" tone="ghost" onClick={() => setDetail(item)}>
                Payload
              </ActionButton>
            </span>
          ),
        }))}
        empty={
          query.isPending
            ? "Loading audit events."
            : action || resourceType
              ? "No audit event matches these filters."
              : "No audit events yet. Events are recorded when this Portfolio, its identities, capital flows, or mappings change."
        }
      />
      <CursorPager
        canPrev={pages.canPrev}
        hasMore={query.data?.page.hasMore ?? false}
        onPrev={pages.prev}
        onNext={() => pages.next(query.data?.page.nextCursor)}
      />
      <Dialog open={Boolean(detail)} title="Audit payload" onClose={() => setDetail(null)}>
        {detail ? (
          <>
            <p className={styles.muted}>
              {detail.action} · {detail.resourceType}
              {detail.resourceId ? ` · ${detail.resourceId}` : ""}
              {detail.requestId ? ` · request ${detail.requestId}` : ""}
            </p>
            {detail.before != null ? (
              <>
                <h2>Before</h2>
                <pre className={styles.jsonBlock}>{safeJson(detail.before)}</pre>
              </>
            ) : null}
            {detail.after != null ? (
              <>
                <h2>After</h2>
                <pre className={styles.jsonBlock}>{safeJson(detail.after)}</pre>
              </>
            ) : null}
            {detail.metadata ? (
              <>
                <h2>Metadata</h2>
                <pre className={styles.jsonBlock}>{safeJson(detail.metadata)}</pre>
              </>
            ) : null}
            {detail.before == null && detail.after == null && !detail.metadata ? (
              <p className={styles.muted}>No payload state recorded.</p>
            ) : null}
            <div className={styles.actions}>
              <ActionButton type="button" tone="ghost" onClick={() => setDetail(null)}>
                Close
              </ActionButton>
            </div>
          </>
        ) : null}
      </Dialog>
    </>
  );
}
