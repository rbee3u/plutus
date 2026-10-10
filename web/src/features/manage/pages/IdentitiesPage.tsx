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
import { StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { Toggle } from "@/shared/ui/FormControls";
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
  const [showArchived, setShowArchived] = useState(false);
  const [dialog, setDialog] = useState<IdentityDialogRequest | null>(null);
  const items = identities.data?.items ?? [];
  const archivedCount = items.filter((item) => item.status === "archived").length;
  const visible = items.filter((item) => showArchived || item.status !== "archived");

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
      <div className={shared.panelHeader} style={{ flexWrap: "wrap", gap: "var(--space-md)" }}>
        <div>
          <h2 style={{ margin: 0 }}>Platform Connections</h2>
          <p className={shared.muted} style={{ margin: "4px 0 0", fontSize: "var(--text-sm)" }}>
            Exchange API connections and credentials used to collect balances, positions, and
            valuations.
          </p>
        </div>
        <div
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "var(--space-md)",
            flexWrap: "wrap",
          }}
        >
          <Toggle checked={showArchived} onCheckedChange={setShowArchived}>
            Show archived ({formatCount(archivedCount)})
          </Toggle>
          <ActionButton
            type="button"
            tone="primary"
            disabled={actions.busyId !== null}
            onClick={() => open({ kind: "create" })}
          >
            New identity
          </ActionButton>
        </div>
      </div>
      <DataTable
        caption="Identities of this Portfolio"
        columns={[
          { key: "name", header: "Identity", nowrap: true },
          { key: "platform", header: "Platform", nowrap: true },
          { key: "connection", header: "Connection" },
          { key: "accounts", header: "Accounts", nowrap: true },
          { key: "lastValidationSucceededAt", header: "Last validated", nowrap: true },
          { key: "lastCollectionFinishedAt", header: "Last collected", nowrap: true },
          { key: "actions", header: "" },
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
            : "Not validated";
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
                  <span className={shared.cellMeta}>
                    {connection.credentialsConfigured ? "credentials stored" : "no credentials"}
                  </span>
                </span>
                {connection.lastErrorClass ? (
                  <span className={shared.cellStack} style={{ marginTop: "2px" }}>
                    <Codes values={[connection.lastErrorClass]} />
                    {connection.lastErrorDetail ? (
                      <span className={shared.cellMeta} style={{ color: "var(--danger)" }}>
                        {connection.lastErrorDetail}
                      </span>
                    ) : null}
                  </span>
                ) : null}
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
                  <>
                    <span className={styles.accountPillDot} aria-hidden="true" />
                    <span>All ({formatCount(selectedCount)})</span>
                  </>
                ) : (
                  <span>
                    {formatCount(selectedCount)}/{formatCount(item.accounts.length)} selected
                  </span>
                )}
              </button>
            ),
            lastValidationSucceededAt: formatInstant(
              connection.lastValidationSucceededAt,
              timezone,
            ),
            lastCollectionFinishedAt: formatInstant(connection.lastCollectionFinishedAt, timezone),
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
            : items.length > 0
              ? "Every identity is archived. Turn on “Show archived” to see them."
              : "No identities yet. Create one for a Platform login, store its credentials, and enable the connection."
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
          Archiving removes {actions.archiveTarget?.displayName ?? "this identity"} from the
          boundary; its history stays. The server refuses while the connection is enabled, so
          disable it first.
        </p>
        <div className={shared.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={actions.busyId !== null}
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
