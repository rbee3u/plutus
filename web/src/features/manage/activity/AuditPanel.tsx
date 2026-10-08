import { useState } from "react";
import { keys, listAuditEvents } from "@/api";
import type { AuditEvent, AuditEventList } from "@/api";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { useCurrentUser } from "@/shared/lib/session";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { safeJson } from "../lib/json";
import { useCursorPages } from "../hooks/useCursorPages";
import styles from "@/features/manage/styles.module.css";
import shared from "@/shared/styles/page.module.css";

const PAGE_SIZE = 50;

export function AuditPanel({
  action,
  resourceType,
  timezone,
  onFilter,
}: {
  action: string;
  resourceType: string;
  timezone?: string;
  onFilter: (next: Record<string, string>) => void;
}) {
  return (
    <>
      <form
        className={shared.compactFilterBar}
        style={{ marginBottom: "var(--space-md)" }}
        onSubmit={(event) => event.preventDefault()}
      >
        <label className={shared.compactFilterField} style={{ maxWidth: "260px" }}>
          <span className={shared.compactFilterLabel}>Action</span>
          <TextInput
            value={action}
            onChange={(event) => onFilter({ action: event.target.value })}
            placeholder="e.g. identity.enable"
          />
        </label>
        <label className={shared.compactFilterField} style={{ maxWidth: "240px" }}>
          <span className={shared.compactFilterLabel}>Resource type</span>
          <TextInput
            value={resourceType}
            onChange={(event) => onFilter({ resourceType: event.target.value })}
            placeholder="e.g. identity"
          />
        </label>
        {action || resourceType ? (
          <ActionButton
            type="button"
            tone="ghost"
            onClick={() => onFilter({ action: "", resourceType: "" })}
          >
            Clear
          </ActionButton>
        ) : null}
      </form>
      <AuditResults
        key={`${action}|${resourceType}`}
        action={action}
        resourceType={resourceType}
        timezone={timezone}
      />
    </>
  );
}

function AuditResults({
  action,
  resourceType,
  timezone,
}: {
  action: string;
  resourceType: string;
  timezone?: string;
}) {
  const me = useCurrentUser();
  const pages = useCursorPages();
  const [detail, setDetail] = useState<AuditEvent | null>(null);
  const query = usePortfolioQuery<AuditEventList>({
    key: (userId, portfolioId) =>
      keys.auditEvents(userId, portfolioId, action, resourceType, pages.cursor),
    load: (portfolioId) =>
      listAuditEvents({
        path: { portfolioId },
        query: {
          action: action || undefined,
          resourceType: resourceType || undefined,
          limit: PAGE_SIZE,
          cursor: pages.cursor || undefined,
        },
      }),
  });

  return (
    <>
      <QueryRecovery query={query} fallback="Audit events could not be loaded." />
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
            <span className={shared.cellStack}>
              <span>{item.resourceType}</span>
              {item.resourceId ? (
                <span className={shared.cellMeta}>#{shortId(item.resourceId)}</span>
              ) : null}
            </span>
          ),
          actor: !item.actorUserId
            ? "system"
            : item.actorUserId === me.data?.id
              ? me.data.username
              : shortId(item.actorUserId),
          actions: (
            <span className={shared.rowActions}>
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
            <p className={shared.muted}>
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
              <p className={shared.muted}>No payload state recorded.</p>
            ) : null}
            <div className={shared.actions}>
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
