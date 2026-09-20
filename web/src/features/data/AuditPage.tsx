import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { listAuditEvents } from "@/api/generated/sdk.gen";
import { safeJson } from "@/features/data/json";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { CursorPager } from "@/shared/ui/Pager";
import { TextInput } from "@/shared/ui/FormControls";
import type { AuditEvent } from "@/api/generated/types.gen";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export function AuditPage() {
  const { portfolioId = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const action = params.get("action") ?? "";
  const resourceType = params.get("resourceType") ?? "";

  function update(next: Record<string, string>) {
    const merged = new URLSearchParams(params);
    for (const [key, value] of Object.entries(next)) {
      if (value) merged.set(key, value);
      else merged.delete(key);
    }
    setParams(merged, { replace: true });
  }

  return (
    <section className={styles.wide}>
      <h2>Audit</h2>
      <p className={styles.lead}>
        Portfolio mutations are recorded with before/after payloads. Secret material is never shown.
      </p>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>Action</span>
          <TextInput value={action} onChange={(event) => update({ action: event.target.value })} />
        </label>
        <label className={styles.field}>
          <span>Resource type</span>
          <TextInput
            value={resourceType}
            onChange={(event) => update({ resourceType: event.target.value })}
          />
        </label>
      </form>
      <AuditResults
        key={`${action}|${resourceType}`}
        portfolioId={portfolioId}
        action={action}
        resourceType={resourceType}
      />
    </section>
  );
}

function AuditResults({
  portfolioId,
  action,
  resourceType,
}: {
  portfolioId: string;
  action: string;
  resourceType: string;
}) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const [cursors, setCursors] = useState<string[]>([""]);
  const [detail, setDetail] = useState<AuditEvent | null>(null);
  const cursor = cursors[cursors.length - 1] ?? "";
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const query = useQuery({
    queryKey: ["audit-events", user.data?.id, portfolioId, action, resourceType, cursor],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAuditEvents({
          path: { portfolioId },
          query: {
            action: action || undefined,
            resourceType: resourceType || undefined,
            limit: PAGE_SIZE,
            cursor: cursor || undefined,
          },
        }),
        redirect,
      ),
  });

  return (
    <>
      {query.error ? (
        <p className={styles.alert} role="alert">
          {query.error instanceof Error ? query.error.message : "Audit events could not be loaded."}
        </p>
      ) : null}
      <DataTable
        caption="Portfolio audit events"
        columns={[
          { key: "occurred", header: "Occurred" },
          { key: "action", header: "Action" },
          { key: "resource", header: "Resource" },
          { key: "actor", header: "Actor" },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          occurred: formatInstant(item.occurredAt),
          action: item.action,
          resource: item.resourceType,
          actor: item.actorUserId ?? "—",
        }))}
      />
      <ul className={styles.flowActions}>
        {(query.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>
              {item.action} · {item.resourceType}
            </span>
            <span className={styles.actions}>
              <ActionButton type="button" onClick={() => setDetail(item)}>
                Before / after
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
      <Dialog open={Boolean(detail)} title="Audit payload" onClose={() => setDetail(null)}>
        {detail ? (
          <>
            <p className={styles.muted}>
              {detail.action} · {detail.resourceType}
              {detail.resourceId ? ` · ${detail.resourceId}` : ""}
            </p>
            <h3>Before</h3>
            <pre className={styles.jsonBlock}>{safeJson(detail.before ?? null)}</pre>
            <h3>After</h3>
            <pre className={styles.jsonBlock}>{safeJson(detail.after ?? null)}</pre>
            <div className={styles.actions}>
              <ActionButton type="button" onClick={() => setDetail(null)}>
                Close
              </ActionButton>
            </div>
          </>
        ) : null}
      </Dialog>
    </>
  );
}
