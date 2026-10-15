import { Plus } from "lucide-react";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatCount } from "@/shared/lib/amount";
import { useRef, useState } from "react";
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
import { formatInstant } from "@/shared/lib/datetime";
import { usePortfolio } from "@/shared/lib/portfolio";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { SegmentedControl } from "@/shared/ui/SegmentedControl";
import { Codes } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { StatusBadge } from "@/shared/ui/Badge";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
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
  const [viewFilter, setViewFilter] = useState<"active" | "archived">("active");
  const [dialog, setDialog] = useState<IdentityDialogRequest | null>(null);
  const items = identities.data?.items ?? [];
  const activeItems = items.filter((item) => item.status !== "archived");
  const archivedItems = items.filter((item) => item.status === "archived");
  const archivedCount = archivedItems.length;

  const activeView = archivedCount > 0 ? viewFilter : "active";
  const visible = activeView === "archived" ? archivedItems : activeItems;

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
      <div className={`${shared.tableToolbar} ${styles.tableToolbar}`}>
        <div className={shared.summaryBar}>
          <span className={shared.summaryCount}>
            {formatCount(visible.length)} {visible.length === 1 ? "identity" : "identities"}
          </span>
        </div>
        <div className={`${shared.toolbarActions} ${styles.toolbarActions}`}>
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
        caption="Identities of this portfolio"
        columns={[
          { key: "platform", header: "Platform", nowrap: true, width: "130px" },
          { key: "name", header: "Identity", nowrap: true, width: "160px" },
          { key: "uid", header: "UID", nowrap: true, width: "150px" },
          { key: "structure", header: "Structure", nowrap: true, width: "140px" },
          { key: "Accounts", header: "Accounts", nowrap: true, width: "110px" },
          { key: "status", header: "Status", width: "140px" },
          {
            key: "lastCollection",
            header: "Last collection",
            nowrap: true,
            width: "170px",
          },
          ...(activeView === "archived"
            ? [
                {
                  key: "archivedAt",
                  header: "Archived at",
                  nowrap: true,
                  width: "170px",
                },
              ]
            : [
                {
                  key: "actions",
                  header: "",
                  width: "80px",
                },
              ]),
        ]}
        rowClassName={(row) => (row.archived ? styles.archivedRow : undefined)}
        rows={visible.map((item) => {
          const archived = item.status === "archived";
          const connection = item.connection;
          const busy = actions.busyId !== null;
          const selectedCount = item.accounts.filter((account) => account.selected).length;
          const extRef = item.externalRef;
          const rawUid = extRef ? (extRef.startsWith("uid-") ? extRef.slice(4) : extRef) : "—";
          const structure =
            [
              structureLabel(platforms.data?.items, item.platform, item.accountStructure),
              marginModeLabel(
                platforms.data?.items,
                item.platform,
                item.accountStructure,
                item.marginMode,
              ),
            ]
              .filter(Boolean)
              .join(" · ") || "—";
          return {
            id: item.id,
            archived,
            platform: <span>{platformLabel(platforms.data?.items, item.platform)}</span>,
            name: <strong>{item.displayName}</strong>,
            uid: <span className={extRef ? shared.nowrap : shared.muted}>{rawUid}</span>,
            structure: (
              <span className={structure === "—" ? shared.muted : shared.nowrap}>{structure}</span>
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
            status: (
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
            ),
            lastCollection: (
              <span
                className={shared.tabularDate}
                title={
                  connection.lastCollectionFinishedAt
                    ? `Validated: ${formatInstant(connection.lastValidationSucceededAt, timezone)}`
                    : connection.lastValidationSucceededAt
                      ? `Never collected (Validated: ${formatInstant(connection.lastValidationSucceededAt, timezone)})`
                      : "Never collected"
                }
              >
                {formatInstant(connection.lastCollectionFinishedAt, timezone)}
              </span>
            ),
            ...(archived
              ? {
                  archivedAt: (
                    <span className={shared.tabularDate}>
                      {formatInstant(item.archivedAt, timezone)}
                    </span>
                  ),
                }
              : {
                  actions: (
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
                }),
          };
        })}
        empty={
          identities.isPending ? (
            <TableEmptyState title="Loading identities..." />
          ) : activeView === "archived" ? (
            <TableEmptyState
              title="No archived identities"
              help="Identities that you archive will appear here."
            />
          ) : items.length > 0 ? (
            <TableEmptyState
              title="No active identities"
              help="All configured identities are currently archived."
            />
          ) : (
            <TableEmptyState
              title="No identities configured yet"
              help="Connect an exchange API key to begin collecting balances and positions."
              action={
                <ActionButton tone="primary" onClick={() => open({ kind: "create" })}>
                  <Plus size={iconSize.sm} aria-hidden="true" /> New identity
                </ActionButton>
              }
            />
          )
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
          Archive {actions.archiveTarget?.displayName ?? "this identity"}? Its accounts will leave
          the portfolio boundary. Existing history is unchanged.
        </p>
        {actions.archiveTarget?.connection.status === "enabled" ? (
          <p className={shared.notice}>
            This connection is currently enabled. Disable the connection before archiving.
          </p>
        ) : null}
        <div className={shared.dialogActions}>
          <ActionButton
            type="button"
            tone="ghost"
            disabled={actions.busyId !== null}
            onClick={() => actions.setArchiveTarget(null)}
          >
            Cancel
          </ActionButton>
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
        </div>
      </Dialog>
    </section>
  );
}
