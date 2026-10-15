import { useEffect, useRef, useState } from "react";
import { keys, listAuditEvents } from "@/api";
import type { AuditEvent, AuditEventList } from "@/api";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { useCurrentUser } from "@/shared/lib/session";
import { capitalize } from "@/shared/lib/text";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { TableFilterHeader, type FilterOption } from "@/shared/ui/TableFilterHeader";
import { safeJson } from "../lib/json";
import { useCursorPages } from "../hooks/useCursorPages";
import styles from "@/features/manage/styles.module.css";
import shared from "@/shared/styles/page.module.css";

const PAGE_SIZE = 50;

const RESOURCE_OPTIONS: FilterOption[] = [
  { value: "asset", label: "Asset" },
  { value: "identity", label: "Identity" },
  { value: "account", label: "Account" },
  { value: "capital_flow", label: "Capital flow" },
  { value: "portfolio", label: "Portfolio" },
];

function resourceLabel(resourceType: string): string {
  return (
    RESOURCE_OPTIONS.find((option) => option.value === resourceType)?.label ??
    capitalize(resourceType.replaceAll("_", " "))
  );
}

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
  const me = useCurrentUser();
  const pages = useCursorPages();
  const [detail, setDetail] = useState<AuditEvent | null>(null);
  const [openFilter, setOpenFilter] = useState<"action" | "resource" | null>(null);
  const [actionDraft, setActionDraft] = useState(action);
  const [prevFilters, setPrevFilters] = useState({ action, resourceType });
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timer.current !== null) clearTimeout(timer.current);
    };
  }, []);

  const handleActionChange = (val: string) => {
    setActionDraft(val);
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      onFilter({ action: val.trim() });
    }, 250);
  };

  const handleActionClear = () => {
    if (timer.current !== null) clearTimeout(timer.current);
    setActionDraft("");
    onFilter({ action: "" });
  };

  function actorDisplay(actorUserId?: string | null) {
    if (!actorUserId) {
      return <span className={shared.cellMeta}>System</span>;
    }
    if (actorUserId === me.data?.id) {
      return <strong>{me.data.username}</strong>;
    }
    return <span className={shared.cellMeta}>#{shortId(actorUserId)}</span>;
  }

  if (prevFilters.action !== action || prevFilters.resourceType !== resourceType) {
    setPrevFilters({ action, resourceType });
    setActionDraft(action);
    pages.reset();
  }

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

  const items = query.data?.items ?? [];
  const hasFilters = Boolean(action || resourceType);

  return (
    <>
      <QueryRecovery query={query} fallback="Audit events could not be loaded." />
      <DataTable
        caption="Portfolio audit events"
        onRowClick={(key) => {
          const item = items.find((i) => i.id === key);
          if (item) setDetail(item);
        }}
        columns={[
          { key: "occurred", header: "Time", nowrap: true, width: "170px" },
          {
            key: "action",
            header: "Action",
            width: "180px",
            headerContent: (
              <TableFilterHeader
                label="Action"
                display={action}
                active={Boolean(action)}
                open={openFilter === "action"}
                type="search"
                searchDraft={actionDraft}
                searchPlaceholder="e.g. catalog.asset.map"
                searchHelp="Filter by exact action or prefix."
                onToggle={() => {
                  setOpenFilter((prev) => (prev === "action" ? null : "action"));
                  setActionDraft(action);
                }}
                onClose={() => setOpenFilter(null)}
                onClear={handleActionClear}
                onSearchChange={handleActionChange}
              />
            ),
          },
          {
            key: "resource",
            header: "Resource",
            headerContent: (
              <TableFilterHeader
                label="Resource"
                display={resourceType ? resourceLabel(resourceType) : ""}
                active={Boolean(resourceType)}
                open={openFilter === "resource"}
                type="select"
                options={RESOURCE_OPTIONS}
                selectedValue={resourceType}
                onToggle={() => setOpenFilter((prev) => (prev === "resource" ? null : "resource"))}
                onClose={() => setOpenFilter(null)}
                onClear={() => {
                  onFilter({ resourceType: "" });
                  setOpenFilter(null);
                }}
                onSelect={(val) => {
                  onFilter({ resourceType: val });
                  setOpenFilter(null);
                }}
              />
            ),
          },
          { key: "actor", header: "Actor", width: "120px" },
          { key: "actions", header: "", nowrap: true, width: "80px" },
        ]}
        rows={items.map((item) => ({
          id: item.id,
          occurred: (
            <span className={shared.tabularDate}>{formatInstant(item.occurredAt, timezone)}</span>
          ),
          action: (
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: "var(--text-xs)",
                letterSpacing: "0.02em",
              }}
            >
              {item.action}
            </span>
          ),
          resource: (
            <span className={shared.cellStack}>
              <span style={{ fontWeight: "var(--weight-medium)" }}>
                {resourceLabel(item.resourceType)}
              </span>
              {item.resourceId ? (
                <span className={shared.cellMeta}>#{shortId(item.resourceId)}</span>
              ) : null}
            </span>
          ),
          actor: actorDisplay(item.actorUserId),
          actions: (
            <span className={shared.rowActions}>
              <ActionButton type="button" tone="ghost" onClick={() => setDetail(item)}>
                Payload
              </ActionButton>
            </span>
          ),
        }))}
        empty={
          query.isPending ? (
            <TableEmptyState title="Loading audit events..." />
          ) : hasFilters ? (
            <TableEmptyState
              title="No audit events match these filters"
              help="Try adjusting your action search or resource filter."
            />
          ) : (
            <TableEmptyState title="No audit events recorded yet" />
          )
        }
      />
      {pages.canPrev || query.data?.page.hasMore || hasFilters ? (
        <div className={shared.headingRow}>
          <CursorPager
            canPrev={pages.canPrev}
            hasMore={query.data?.page.hasMore ?? false}
            onPrev={pages.prev}
            onNext={() => pages.next(query.data?.page.nextCursor)}
          />
          {hasFilters ? (
            <ActionButton
              type="button"
              tone="ghost"
              onClick={() => {
                setActionDraft("");
                onFilter({ action: "", resourceType: "" });
              }}
            >
              Clear filters
            </ActionButton>
          ) : null}
        </div>
      ) : null}
      <Dialog
        open={Boolean(detail)}
        title={detail ? `Audit event · ${detail.action}` : "Audit event"}
        size="lg"
        onClose={() => setDetail(null)}
      >
        {detail ? (
          <>
            <div className={styles.payloadMetaGrid}>
              <div className={styles.payloadMetaItem}>
                <span className={styles.payloadMetaLabel}>Time</span>
                <span className={styles.payloadMetaValue}>
                  {formatInstant(detail.occurredAt, timezone)}
                </span>
              </div>
              <div className={styles.payloadMetaItem}>
                <span className={styles.payloadMetaLabel}>Actor</span>
                <span className={styles.payloadMetaValue}>{actorDisplay(detail.actorUserId)}</span>
              </div>
              <div className={styles.payloadMetaItem}>
                <span className={styles.payloadMetaLabel}>Resource</span>
                <span className={styles.payloadMetaValue}>
                  {resourceLabel(detail.resourceType)}
                  {detail.resourceId ? ` · #${shortId(detail.resourceId)}` : ""}
                </span>
              </div>
              <div className={styles.payloadMetaItem}>
                <span className={styles.payloadMetaLabel}>Request ID</span>
                <span className={styles.payloadMetaValue}>
                  {detail.requestId ? `#${shortId(detail.requestId)}` : "—"}
                </span>
              </div>
            </div>
            {detail.before != null && detail.after != null ? (
              <div className={styles.payloadDiffGrid}>
                <div className={styles.payloadSection}>
                  <span className={styles.payloadSectionLabel}>Before state</span>
                  <pre className={styles.jsonBlock}>{safeJson(detail.before)}</pre>
                </div>
                <div className={styles.payloadSection}>
                  <span className={styles.payloadSectionLabel}>After state</span>
                  <pre className={styles.jsonBlock}>{safeJson(detail.after)}</pre>
                </div>
              </div>
            ) : (
              <>
                {detail.before != null ? (
                  <div className={styles.payloadSection}>
                    <span className={styles.payloadSectionLabel}>Before state</span>
                    <pre className={styles.jsonBlock}>{safeJson(detail.before)}</pre>
                  </div>
                ) : null}
                {detail.after != null ? (
                  <div className={styles.payloadSection}>
                    <span className={styles.payloadSectionLabel}>After state</span>
                    <pre className={styles.jsonBlock}>{safeJson(detail.after)}</pre>
                  </div>
                ) : null}
              </>
            )}
            {detail.metadata ? (
              <div className={styles.payloadSection}>
                <span className={styles.payloadSectionLabel}>Metadata</span>
                <pre className={styles.jsonBlock}>{safeJson(detail.metadata)}</pre>
              </div>
            ) : null}
            {detail.before == null && detail.after == null && !detail.metadata ? (
              <p className={shared.muted}>No payload state recorded.</p>
            ) : null}
            <div className={shared.dialogActions}>
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
