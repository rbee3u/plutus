import { useQueryClient } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { useRef, useState } from "react";
import { useParams } from "react-router";
import {
  archiveIdentity,
  disableIdentityConnection,
  enableIdentityConnection,
  validateIdentityConnection,
} from "@/api/generated/sdk.gen";
import type { Identity, ValidationOutcome } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import {
  IdentityDialog,
  type IdentityDialogInput,
  type IdentityDialogRequest,
} from "@/features/data/IdentityDialogs";
import { identityEtag, useIdentities } from "@/features/data/useIdentities";
import { accountModeLabel, platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { formatInstant } from "@/shared/lib/datetime";
import { iconSize } from "@/shared/icons/iconSizes";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { Toggle } from "@/shared/ui/FormControls";
import { IconButton } from "@/shared/ui/IconButton";
import { Codes } from "@/shared/ui/IssueList";
import { Menu, MenuItem, MenuSeparator, useMenu } from "@/shared/ui/Menu";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const CONFLICT = "The identity changed in another session. The list was reloaded; retry.";

function isOutcome(value: unknown): value is ValidationOutcome {
  return (
    typeof value === "object" &&
    value !== null &&
    "ok" in value &&
    "identity" in value &&
    "validatedAt" in value
  );
}

function sentence(text: string): string {
  return /[.!?]$/.test(text) ? text : `${text}.`;
}

function outcomeText(outcome: ValidationOutcome): string {
  if (outcome.ok) {
    return `ok${outcome.externalRef ? ` (account ${outcome.externalRef})` : ""}`;
  }
  return `${outcome.errorClass ?? "failed"}${outcome.errorDetail ? `: ${outcome.errorDetail}` : ""}`;
}

export function IdentitiesPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const identities = useIdentities(portfolioId);
  const platforms = usePlatforms();
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.item.timezone;
  const nonce = useRef(0);
  const [showArchived, setShowArchived] = useState(false);
  const [dialog, setDialog] = useState<IdentityDialogRequest | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<Identity | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const items = identities.data?.items ?? [];
  const archivedCount = items.filter((item) => item.status === "archived").length;
  const visible = items.filter((item) => showArchived || item.status !== "archived");

  async function invalidate() {
    await queryClient.invalidateQueries({ queryKey: ["identities", userId, portfolioId] });
  }

  function open(input: IdentityDialogInput) {
    setError(null);
    setNotice(null);
    nonce.current += 1;
    setDialog({ ...input, nonce: nonce.current });
  }

  async function runAction(
    item: Identity,
    action: () => Promise<{ status: number | undefined; message: string | null; error?: unknown }>,
    fallback: string,
  ) {
    setBusyId(item.id);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const outcome = await action();
      if (outcome.status === 401) {
        redirect();
        return;
      }
      if (outcome.status === 412) {
        setError(CONFLICT);
        await invalidate();
        return;
      }
      if (outcome.message === null) {
        setError(problemMessage(outcome.error, fallback));
        return;
      }
      setNotice(outcome.message);
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, fallback));
    } finally {
      setBusyId(null);
    }
  }

  function validate(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await validateIdentityConnection({
          path: { portfolioId, identityId: item.id },
        });
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Validation of ${item.displayName}: ${sentence(outcomeText(result.data))} Connection is ${result.data.identity.connection.status}.`,
        };
      },
      "Validation could not run.",
    );
  }

  function enable(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await enableIdentityConnection({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        });
        if (result.response?.status === 422 && isOutcome(result.error)) {
          await invalidate();
          return {
            status: 422,
            message: null,
            error: new Error(
              `Enable failed for ${item.displayName}: ${sentence(outcomeText(result.error))} Connection is now ${result.error.identity.connection.status}.`,
            ),
          };
        }
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Connection enabled for ${item.displayName}: ${sentence(outcomeText(result.data))} It joins the next Collection Cycle.`,
        };
      },
      "The connection could not be enabled.",
    );
  }

  function disable(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await disableIdentityConnection({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        });
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Connection disabled for ${item.displayName}. Its accounts leave the boundary at the next Collection Cycle.`,
        };
      },
      "The connection could not be disabled.",
    );
  }

  async function confirmArchive() {
    const item = archiveTarget;
    if (!item) {
      return;
    }
    await runAction(
      item,
      async () => {
        const result = await archiveIdentity({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        });
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        setArchiveTarget(null);
        return {
          status: result.response?.status,
          message: `Identity ${item.displayName} archived.`,
        };
      },
      "The identity could not be archived.",
    );
  }

  return (
    <section className={styles.wide}>
      <div className={styles.headingRow}>
        <p className={styles.lead}>
          One Identity per Platform login: store read-only credentials, validate them, then enable
          the connection so its selected accounts join every Collection Cycle.
        </p>
        <ActionButton
          type="button"
          tone="primary"
          disabled={busyId !== null}
          onClick={() => open({ kind: "create" })}
        >
          New identity
        </ActionButton>
      </div>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      ) : null}
      <QueryRecovery
        error={identities.error}
        hasData={Boolean(identities.data)}
        busy={identities.isFetching}
        fallback="Identities could not be loaded."
        onRetry={() => {
          void identities.refetch();
        }}
      />
      <div className={styles.toolbar}>
        <Toggle checked={showArchived} onCheckedChange={setShowArchived}>
          Show archived ({String(archivedCount)})
        </Toggle>
      </div>
      <DataTable
        caption="Identities of this Portfolio"
        columns={[
          { key: "name", header: "Identity", nowrap: true },
          { key: "platform", header: "Platform" },
          { key: "connection", header: "Connection", nowrap: true },
          { key: "accounts", header: "Accounts", nowrap: true },
          { key: "lastValidatedAt", header: "Last validated", nowrap: true },
          { key: "lastCollectedAt", header: "Last collected", nowrap: true },
          { key: "lastError", header: "Last error" },
          { key: "actions", header: "" },
        ]}
        rowClassName={(row) => (row.archived ? styles.archivedRow : undefined)}
        rows={visible.map((item) => {
          const archived = item.status === "archived";
          const connection = item.connection;
          const busy = busyId !== null;
          const selectedCount = item.accounts.filter((account) => account.selected).length;
          return {
            id: item.id,
            archived,
            name: (
              <span className={styles.cellStack}>
                <strong>{item.displayName}</strong>
                <span className={styles.cellMeta}>
                  {archived
                    ? `archived ${formatInstant(item.archivedAt, timezone)}`
                    : (item.externalRef ?? "not validated yet")}
                </span>
              </span>
            ),
            platform: (
              <span className={styles.cellStack}>
                <span>{platformLabel(platforms.data?.items, item.platform)}</span>
                <span className={styles.cellMeta}>
                  {accountModeLabel(platforms.data?.items, item.platform, item.accountMode)}
                </span>
              </span>
            ),
            connection: (
              <span className={styles.cellStack}>
                <span>
                  <StatusBadge value={connection.status} />
                </span>
                <span className={styles.cellMeta}>
                  {connection.credentialsConfigured ? "credentials stored" : "no credentials"}
                </span>
              </span>
            ),
            accounts: `${String(selectedCount)}/${String(item.accounts.length)} selected`,
            lastValidatedAt: formatInstant(connection.lastValidatedAt, timezone),
            lastCollectedAt: formatInstant(connection.lastCollectedAt, timezone),
            lastError: connection.lastErrorClass ? (
              <span className={styles.cellStack}>
                <Codes values={[connection.lastErrorClass]} />
                {connection.lastErrorDetail ? (
                  <span className={styles.cellMeta}>{connection.lastErrorDetail}</span>
                ) : null}
              </span>
            ) : (
              "—"
            ),
            actions: archived ? (
              <span className={styles.cellMeta}>archived</span>
            ) : (
              <IdentityActions
                item={item}
                busy={busy}
                onEdit={() => open({ kind: "edit", item })}
                onCredentials={() => open({ kind: "credentials", item })}
                onValidate={() => void validate(item)}
                onEnable={() => void enable(item)}
                onDisable={() => void disable(item)}
                onArchive={() => {
                  setError(null);
                  setNotice(null);
                  setArchiveTarget(item);
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
          setNotice(message);
          await invalidate();
        }}
      />
      <Dialog
        open={archiveTarget !== null}
        title="Archive identity"
        onClose={() => {
          if (busyId === null) setArchiveTarget(null);
        }}
      >
        <p className={styles.lead}>
          Archiving removes {archiveTarget?.displayName ?? "this identity"} from the boundary; its
          history stays. The server refuses while the connection is enabled, so disable it first.
        </p>
        <div className={styles.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={busyId !== null}
            onClick={() => void confirmArchive()}
          >
            Archive
          </ActionButton>
          <ActionButton
            type="button"
            tone="ghost"
            disabled={busyId !== null}
            onClick={() => setArchiveTarget(null)}
          >
            Cancel
          </ActionButton>
        </div>
      </Dialog>
    </section>
  );
}

interface IdentityActionsProps {
  item: Identity;
  busy: boolean;
  onEdit: () => void;
  onCredentials: () => void;
  onValidate: () => void;
  onEnable: () => void;
  onDisable: () => void;
  onArchive: () => void;
}

/**
 * The next step for the connection sits in the row; everything else waits in
 * a menu. An enabled connection needs nothing, so its row shows only the menu.
 */
function IdentityActions({
  item,
  busy,
  onEdit,
  onCredentials,
  onValidate,
  onEnable,
  onDisable,
  onArchive,
}: IdentityActionsProps) {
  const menu = useMenu();
  const connection = item.connection;
  const next = !connection.credentialsConfigured
    ? { label: "Add credentials", run: onCredentials }
    : connection.status === "invalid"
      ? { label: "Validate", run: onValidate }
      : connection.status === "enabled"
        ? null
        : { label: "Enable", run: onEnable };

  function pick(action: () => void, restoreFocus: boolean) {
    menu.close(restoreFocus);
    action();
  }

  return (
    <span className={styles.rowActions}>
      {next ? (
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={next.run}>
          {next.label}
        </ActionButton>
      ) : null}
      <IconButton
        ref={(node) => menu.setAnchor(node)}
        label={`More actions for ${item.displayName}`}
        aria-haspopup="menu"
        aria-expanded={menu.open}
        disabled={busy}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <MoreHorizontal size={iconSize.sm} aria-hidden="true" />
      </IconButton>
      <Menu menu={menu} label={`Actions for ${item.displayName}`}>
        <MenuItem onClick={() => pick(onEdit, false)}>Edit</MenuItem>
        <MenuItem onClick={() => pick(onCredentials, false)}>Credentials</MenuItem>
        <MenuItem
          disabled={!connection.credentialsConfigured}
          onClick={() => pick(onValidate, true)}
        >
          Validate
        </MenuItem>
        {connection.status === "enabled" ? (
          <MenuItem onClick={() => pick(onDisable, true)}>Disable</MenuItem>
        ) : (
          <MenuItem
            disabled={!connection.credentialsConfigured}
            onClick={() => pick(onEnable, true)}
          >
            Enable
          </MenuItem>
        )}
        <MenuSeparator />
        <MenuItem danger onClick={() => pick(onArchive, false)}>
          Archive
        </MenuItem>
      </Menu>
    </span>
  );
}
