import { Plus } from "lucide-react";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatCount } from "@/shared/lib/amount";
import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import {
  IdentityDialog,
  type IdentityDialogInput,
  type IdentityDialogRequest,
} from "@/features/manage/dialogs/IdentityDialog";
import { IdentityActions } from "../components/IdentityActions";
import { useIdentityActions } from "../hooks/useIdentityActions";
import { useIdentities } from "@/shared/lib/useIdentities";
import {
  marginModeLabel,
  platformLabel,
  structureLabel,
  usePlatforms,
} from "@/shared/lib/usePlatforms";
import { formatInstant, formatRelativeTime } from "@/shared/lib/datetime";
import { usePortfolio } from "@/shared/lib/portfolio";
import { ActionButton } from "@/shared/ui/ActionButton";
import { StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { SegmentedControl } from "@/shared/ui/SegmentedControl";
import { Codes } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/manage/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function IdentitiesPage() {
  const { portfolioId = "" } = useParams();
  const identities = useIdentities(portfolioId);
  const platforms = usePlatforms();
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.timezone;
  const actions = useIdentityActions(portfolioId);
  const nonce = useRef(0);
  const [now, setNow] = useState(() => Date.now());
  const [viewFilter, setViewFilter] = useState<"active" | "archived">("active");

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(timer);
  }, []);
  const [dialog, setDialog] = useState<IdentityDialogRequest | null>(null);
  const items = identities.data?.items ?? [];
  const activeItems = items.filter((item) => item.status !== "archived");
  const archivedItems = items.filter((item) => item.status === "archived");
  const archivedCount = archivedItems.length;

  const activeView = archivedCount > 0 ? viewFilter : "active";
  const visible = activeView === "archived" ? archivedItems : activeItems;

  const invalidCount = activeItems.filter(
    (item) => item.connection.status === "invalid" || Boolean(item.connection.lastErrorClass),
  ).length;
  const unconfiguredCount = activeItems.filter(
    (item) => !item.connection.credentialsConfigured,
  ).length;

  function open(input: IdentityDialogInput) {
    actions.clearMessages();
    nonce.current += 1;
    setDialog({ ...input, nonce: nonce.current });
  }

  return (
    <section className={shared.wide}>
      {actions.error ? (
        <p className={shared.alert} role="alert">
          {actions.error}
        </p>
      ) : null}
      {actions.notice ? (
        <p className={shared.notice} role="status">
          {actions.notice}
        </p>
      ) : null}
      <QueryRecovery query={identities} fallback="Identities could not be loaded." />
      <QueryRecovery query={platforms} fallback="Platforms could not be loaded." />
      <div className={styles.tableToolbar}>
        <div className={styles.summaryBar}>
          <span className={styles.summaryCount}>
            {formatCount(activeItems.length)}{" "}
            {activeItems.length === 1 ? "connection" : "connections"}
          </span>
          {activeItems.length > 0 ? (
            <>
              <span className={styles.summaryDot} aria-hidden="true">
                ·
              </span>
              {invalidCount > 0 ? (
                <span className={styles.summaryStatusAttention}>
                  <span className={styles.statusDotDanger} aria-hidden="true" />
                  {formatCount(invalidCount)}{" "}
                  {invalidCount === 1 ? "needs attention" : "need attention"}
                </span>
              ) : unconfiguredCount > 0 ? (
                <span className={styles.summaryStatusNeutral}>
                  {formatCount(unconfiguredCount)} unconfigured
                </span>
              ) : (
                <span className={styles.summaryStatusHealthy}>
                  <span className={styles.statusDotSuccess} aria-hidden="true" />
                  All healthy
                </span>
              )}
            </>
          ) : null}
        </div>
        <div className={styles.toolbarActions}>
          {archivedCount > 0 ? (
            <SegmentedControl
              size="sm"
              label="Filter identities by status"
              value={activeView}
              onChange={setViewFilter}
              options={[
                { value: "active", label: `Active (${formatCount(activeItems.length)})` },
                { value: "archived", label: `Archived (${formatCount(archivedCount)})` },
              ]}
            />
          ) : null}
          <ActionButton
            type="button"
            tone="primary"
            disabled={actions.busyId !== null}
            onClick={() => open({ kind: "create" })}
          >
            <Plus size={iconSize.sm} aria-hidden="true" />
            <span>New identity</span>
          </ActionButton>
        </div>
      </div>
      <DataTable
        caption="Identities of this Portfolio"
        columns={[
          { key: "name", header: "Identity", nowrap: true, width: "190px" },
          { key: "platform", header: "Platform", nowrap: true, width: "150px" },
          { key: "connection", header: "Connection" },
          { key: "accounts", header: "Accounts", nowrap: true, width: "120px" },
          {
            key: "lastSynced",
            header: "Last synced",
            nowrap: true,
            width: "120px",
          },
          { key: "actions", header: "", width: "110px" },
        ]}
        rowClassName={(row) => (row.archived ? styles.archivedRow : undefined)}
        rows={visible.map((item) => {
          const archived = item.status === "archived";
          const connection = item.connection;
          const busy = actions.busyId !== null;
          const selectedCount = item.accounts.filter((account) => account.selected).length;
          const extRef = item.externalRef;
          const refDisplay = extRef
            ? extRef.startsWith("uid-")
              ? `UID: ${extRef.slice(4)}`
              : `UID: ${extRef}`
            : "—";
          return {
            id: item.id,
            archived,
            name: (
              <span className={shared.cellStack}>
                <strong>{item.displayName}</strong>
                <span className={shared.cellMeta}>
                  {archived ? `archived ${formatInstant(item.archivedAt, timezone)}` : refDisplay}
                </span>
              </span>
            ),
            platform: (
              <span className={shared.cellStack}>
                <span>{platformLabel(platforms.data?.items, item.platform)}</span>
                <span className={shared.cellMeta}>
                  {[
                    structureLabel(platforms.data?.items, item.platform, item.accountStructure),
                    marginModeLabel(
                      platforms.data?.items,
                      item.platform,
                      item.accountStructure,
                      item.marginMode,
                    ),
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </span>
            ),
            connection: (
              <span className={shared.cellStack}>
                <span
                  style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-xs)" }}
                >
                  <StatusBadge value={connection.status} />
                  {connection.lastErrorClass ? (
                    connection.lastErrorDetail ? (
                      <HoverDetails
                        className={styles.errorTrigger}
                        name={`Error details for ${item.displayName}`}
                        label={<Codes values={[connection.lastErrorClass]} />}
                      >
                        <div className={styles.errorHoverContent}>
                          <span className={styles.errorHoverTitle}>
                            [{connection.lastErrorClass}]
                          </span>
                          <span className={styles.errorHoverDetail}>
                            {connection.lastErrorDetail}
                          </span>
                        </div>
                      </HoverDetails>
                    ) : (
                      <Codes values={[connection.lastErrorClass]} />
                    )
                  ) : null}
                </span>
              </span>
            ),
            accounts: archived ? (
              <span className={shared.muted}>
                {formatCount(selectedCount)}/{formatCount(item.accounts.length)}
              </span>
            ) : (
              <button
                type="button"
                className={styles.accountPillBtn}
                disabled={busy}
                onClick={() => open({ kind: "edit", item })}
                title={`Configure selected accounts for ${item.displayName}`}
              >
                {selectedCount === item.accounts.length && item.accounts.length > 0 ? (
                  <span>All ({formatCount(selectedCount)})</span>
                ) : (
                  <span>
                    {formatCount(selectedCount)}/{formatCount(item.accounts.length)}
                  </span>
                )}
              </button>
            ),
            lastSynced: (
              <span
                className={shared.tabularDate}
                title={
                  connection.lastCollectionFinishedAt
                    ? `Collected: ${formatInstant(connection.lastCollectionFinishedAt, timezone)}\nValidated: ${formatInstant(connection.lastValidationSucceededAt, timezone)}`
                    : connection.lastValidationSucceededAt
                      ? `Never collected (Validated: ${formatInstant(connection.lastValidationSucceededAt, timezone)})`
                      : "Never synced"
                }
              >
                {formatRelativeTime(connection.lastCollectionFinishedAt, now, timezone)}
              </span>
            ),
            actions: archived ? (
              <span className={shared.cellMeta}>archived</span>
            ) : (
              <IdentityActions
                item={item}
                busy={busy}
                onEdit={() => open({ kind: "edit", item })}
                onCredentials={() => open({ kind: "credentials", item })}
                onValidate={() => void actions.validate(item)}
                onEnable={() => void actions.enable(item)}
                onDisable={() => void actions.disable(item)}
                onArchive={() => {
                  actions.clearMessages();
                  actions.setArchiveTarget(item);
                }}
              />
            ),
          };
        })}
        empty={
          identities.isPending
            ? "Loading identities."
            : activeView === "archived"
              ? "No archived identities."
              : items.length > 0
                ? "No active identities."
                : "No identities configured in this portfolio."
        }
      />
      <IdentityDialog
        portfolioId={portfolioId}
        request={dialog}
        platforms={platforms.data?.items ?? []}
        onClose={() => setDialog(null)}
        onSaved={async (message) => {
          setDialog(null);
          actions.notify(message);
          await actions.invalidate();
        }}
      />
      <Dialog
        open={actions.archiveTarget !== null}
        title="Archive identity"
        onClose={() => {
          if (actions.busyId === null) actions.setArchiveTarget(null);
        }}
      >
        <p className={shared.lead}>
          Archive {actions.archiveTarget?.displayName ?? "this identity"}? Active data collection
          will stop immediately. Historical balances, transactions, and valuations remain fully
          preserved.
        </p>
        {actions.archiveTarget?.connection.status === "enabled" ? (
          <p className={shared.notice}>
            This connection is currently enabled. Disable the connection before archiving.
          </p>
        ) : null}
        <div className={shared.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={
              actions.busyId !== null || actions.archiveTarget?.connection.status === "enabled"
            }
            onClick={() => void actions.confirmArchive()}
          >
            Archive
          </ActionButton>
          <ActionButton
            type="button"
            tone="ghost"
            disabled={actions.busyId !== null}
            onClick={() => actions.setArchiveTarget(null)}
          >
            Cancel
          </ActionButton>
        </div>
      </Dialog>
    </section>
  );
}
