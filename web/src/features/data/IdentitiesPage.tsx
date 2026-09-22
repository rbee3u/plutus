import { Fragment, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, ChevronUp } from "lucide-react";
import {
  archiveIdentity,
  deleteConnection,
  getConnection,
  getIdentity,
  listConnections,
  listIdentities,
  listObservations,
} from "@/api/generated/sdk.gen";
import type { Connection, Identity } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import {
  ConnectionFormDialog,
  type ConnectionDialogRequest,
} from "@/features/data/ConnectionFormDialog";
import { IdentityFormDialog, type IdentityDialogRequest } from "@/features/data/IdentityFormDialog";
import { platformLabel } from "@/features/data/platforms";
import { formatInstant } from "@/shared/lib/datetime";
import { iconSize } from "@/shared/icons/iconSizes";
import { readEtag, unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { IconButton } from "@/shared/ui/IconButton";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Select } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const identityColumnCount = 9;

function connectionSummary(rows: Connection[]): string {
  if (rows.length === 0) {
    return "None";
  }
  const enabled = rows.filter((row) => row.status === "enabled").length;
  const invalid = rows.filter((row) => row.status === "invalid").length;
  const base = `${String(enabled)}/${String(rows.length)} enabled`;
  if (invalid === 0) {
    return base;
  }
  return `${base} · ${String(invalid)} invalid`;
}

function secondsLabel(value: number | undefined): string {
  return value != null ? `${String(value)}s` : "—";
}

function compareIdentities(left: Identity, right: Identity, direction: "asc" | "desc"): number {
  const platform = platformLabel(left.platformId).localeCompare(platformLabel(right.platformId));
  if (platform !== 0) {
    return direction === "asc" ? platform : -platform;
  }
  return left.displayName.localeCompare(right.displayName);
}

export function IdentitiesPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const nonce = useRef(0);
  const [platformFilter, setPlatformFilter] = useState("all");
  const [platformSort, setPlatformSort] = useState<"asc" | "desc">("asc");
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [identityRequest, setIdentityRequest] = useState<IdentityDialogRequest | null>(null);
  const [connectionRequest, setConnectionRequest] = useState<ConnectionDialogRequest | null>(null);
  const [archiveId, setArchiveId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const identities = useQuery({
    queryKey: ["identities", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await listIdentities({ path: { portfolioId } }), redirect),
  });
  const connections = useQuery({
    queryKey: ["connections", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await listConnections({ path: { portfolioId } }), redirect),
  });
  const identityItems = identities.data?.items;
  const connectionItems = connections.data?.items;
  const connectionsByIdentity = useMemo(() => {
    const grouped = new Map<string, Connection[]>();
    for (const item of connectionItems ?? []) {
      const current = grouped.get(item.identityId);
      if (current) {
        current.push(item);
      } else {
        grouped.set(item.identityId, [item]);
      }
    }
    return grouped;
  }, [connectionItems]);
  const platformChoices = useMemo(() => {
    const seen = new Map<string, string>();
    for (const item of identityItems ?? []) {
      if (!seen.has(item.platformId)) {
        seen.set(item.platformId, platformLabel(item.platformId));
      }
    }
    return [...seen.entries()]
      .map(([id, label]) => ({ id, label }))
      .sort((left, right) => left.label.localeCompare(right.label));
  }, [identityItems]);
  const visibleIdentities = useMemo(() => {
    return (identityItems ?? [])
      .filter((item) => platformFilter === "all" || item.platformId === platformFilter)
      .slice()
      .sort((left, right) => compareIdentities(left, right, platformSort));
  }, [identityItems, platformFilter, platformSort]);

  function nextNonce() {
    nonce.current += 1;
    return nonce.current;
  }

  function identityName(identityId: string) {
    return identityItems?.find((item) => item.id === identityId)?.displayName ?? identityId;
  }

  async function invalidateIdentities() {
    await queryClient.invalidateQueries({ queryKey: ["identities", user.data?.id, portfolioId] });
  }

  async function invalidateConnections() {
    await queryClient.invalidateQueries({ queryKey: ["connections", user.data?.id, portfolioId] });
  }

  function changePlatform(next: string) {
    setPlatformFilter(next);
    const allowed = new Set(
      (identityItems ?? [])
        .filter((item) => next === "all" || item.platformId === next)
        .map((item) => item.id),
    );
    setExpanded((current) => {
      const nextSet = new Set([...current].filter((id) => allowed.has(id)));
      return nextSet.size === current.size ? current : nextSet;
    });
  }

  function toggleExpanded(identityId: string) {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(identityId)) {
        next.delete(identityId);
      } else {
        next.add(identityId);
      }
      return next;
    });
  }

  function openIdentityCreate() {
    setError(null);
    setNotice(null);
    setConnectionRequest(null);
    setIdentityRequest({ nonce: nextNonce(), mode: "create" });
  }

  async function openIdentityEdit(identityId: string) {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const result = await getIdentity({ path: { portfolioId, identityId } });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(result, redirect);
      const observed = await listObservations({
        path: { portfolioId },
        query: { identityId, limit: 1 },
      });
      if (observed.response?.status === 401) {
        redirect();
        return;
      }
      setConnectionRequest(null);
      setIdentityRequest({
        nonce: nextNonce(),
        mode: "edit",
        item,
        etag: readEtag(result.response, `"identity:${String(item.version)}"`),
        frozen: (observed.data?.items.length ?? 0) > 0,
      });
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be loaded."));
    } finally {
      setBusy(false);
    }
  }

  async function openConnectionEdit(connectionId: string, name: string) {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const result = await getConnection({ path: { portfolioId, connectionId } });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(result, redirect);
      setIdentityRequest(null);
      setConnectionRequest({
        nonce: nextNonce(),
        mode: "edit",
        item,
        etag: readEtag(result.response, `"connection:${String(item.version)}"`),
        identityName: name,
      });
    } catch (cause) {
      setError(problemMessage(cause, "The connection could not be loaded."));
    } finally {
      setBusy(false);
    }
  }

  function openConnectionCreate(identityId: string) {
    setError(null);
    setNotice(null);
    setIdentityRequest(null);
    setConnectionRequest({
      nonce: nextNonce(),
      mode: "create",
      identityId,
      identityName: identityName(identityId),
    });
  }

  async function confirmArchive() {
    if (!archiveId) {
      return;
    }
    const identityId = archiveId;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const current = await getIdentity({ path: { portfolioId, identityId } });
      if (current.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(current, redirect);
      const match = readEtag(current.response, `"identity:${String(item.version)}"`);
      await refreshCsrf();
      const result = await archiveIdentity({
        path: { portfolioId, identityId },
        headers: { "If-Match": match },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setArchiveId(null);
        setError("The identity changed in another session. Reload before archiving.");
        await invalidateIdentities();
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The identity could not be archived."));
        return;
      }
      if (identityRequest?.mode === "edit" && identityRequest.item.id === identityId) {
        setIdentityRequest(null);
      }
      setArchiveId(null);
      setNotice("Identity archived.");
      await invalidateIdentities();
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be archived."));
    } finally {
      setBusy(false);
    }
  }

  async function confirmDelete() {
    if (!deleteId) {
      return;
    }
    const connectionId = deleteId;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const current = await getConnection({ path: { portfolioId, connectionId } });
      if (current.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(current, redirect);
      const match = readEtag(current.response, `"connection:${String(item.version)}"`);
      await refreshCsrf();
      const result = await deleteConnection({
        path: { portfolioId, connectionId },
        headers: { "If-Match": match },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setDeleteId(null);
        setError("The connection changed in another session. Reload before deleting.");
        await invalidateConnections();
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The connection could not be deleted."));
        return;
      }
      if (connectionRequest?.mode === "edit" && connectionRequest.item.id === connectionId) {
        setConnectionRequest(null);
      }
      setDeleteId(null);
      setNotice("Connection deleted.");
      await invalidateConnections();
    } catch (cause) {
      setError(problemMessage(cause, "The connection could not be deleted."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.wide}>
      <div className={styles.headingRow}>
        <div className={styles.headingCopy}>
          <h2>Identities</h2>
          <p className={styles.lead}>
            Platform login subjects stay in this Portfolio. Expand an identity to see its
            Connections. After the first Observation, identity fields other than name and status
            freeze.
          </p>
        </div>
        <ActionButton type="button" tone="primary" disabled={busy} onClick={openIdentityCreate}>
          New identity
        </ActionButton>
      </div>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <QueryRecovery
        error={identities.error}
        hasData={Boolean(identities.data)}
        busy={identities.isFetching}
        fallback="Identities could not be loaded."
        onRetry={() => {
          void identities.refetch();
        }}
      />
      <QueryRecovery
        error={connections.error}
        hasData={Boolean(connections.data)}
        busy={connections.isFetching}
        fallback="Connections could not be loaded."
        onRetry={() => {
          void connections.refetch();
        }}
      />
      <div className={styles.toolbar}>
        <label className={`${styles.field} ${styles.toolbarField}`}>
          <span>Platform</span>
          <Select value={platformFilter} onChange={(event) => changePlatform(event.target.value)}>
            <option value="all">All</option>
            {platformChoices.map((item) => (
              <option key={item.id} value={item.id}>
                {item.label}
              </option>
            ))}
          </Select>
        </label>
      </div>
      <div className={styles.tableWrap}>
        <table>
          <caption>Portfolio identities</caption>
          <thead>
            <tr>
              <th scope="col">
                <span className={styles.srOnly}>Expand</span>
              </th>
              <th scope="col">Name</th>
              <th scope="col" aria-sort={platformSort === "asc" ? "ascending" : "descending"}>
                <ActionButton
                  type="button"
                  tone="ghost"
                  className={styles.sortButton}
                  onClick={() => setPlatformSort((current) => (current === "asc" ? "desc" : "asc"))}
                >
                  Platform
                  {platformSort === "asc" ? (
                    <ChevronUp size={iconSize.xs} aria-hidden="true" />
                  ) : (
                    <ChevronDown size={iconSize.xs} aria-hidden="true" />
                  )}
                </ActionButton>
              </th>
              <th scope="col">Kind</th>
              <th scope="col">Account mode</th>
              <th scope="col">Status</th>
              <th scope="col">External ref</th>
              <th scope="col">Connections</th>
              <th scope="col">Actions</th>
            </tr>
          </thead>
          <tbody>
            {visibleIdentities.length === 0 ? (
              <tr>
                <td colSpan={identityColumnCount}>No rows.</td>
              </tr>
            ) : (
              visibleIdentities.map((item) => {
                const open = expanded.has(item.id);
                const rows = connectionItems ? (connectionsByIdentity.get(item.id) ?? []) : null;
                const detailId = `identity-connections-${item.id}`;
                return (
                  <Fragment key={item.id}>
                    <tr
                      className={open ? styles.openRow : styles.clickRow}
                      onClick={(event) => {
                        if (event.target instanceof Element && event.target.closest("button")) {
                          return;
                        }
                        toggleExpanded(item.id);
                      }}
                    >
                      <td>
                        <IconButton
                          label={
                            open
                              ? `Collapse connections for ${item.displayName}`
                              : `Expand connections for ${item.displayName}`
                          }
                          aria-expanded={open}
                          aria-controls={open ? detailId : undefined}
                          onClick={() => toggleExpanded(item.id)}
                        >
                          <ChevronRight
                            size={iconSize.sm}
                            aria-hidden="true"
                            className={open ? styles.chevronOpen : styles.chevron}
                          />
                        </IconButton>
                      </td>
                      <td>{item.displayName}</td>
                      <td>{platformLabel(item.platformId)}</td>
                      <td>{item.kind}</td>
                      <td>{item.accountMode}</td>
                      <td>{item.status}</td>
                      <td>{item.externalRef}</td>
                      <td>{rows === null ? "—" : connectionSummary(rows)}</td>
                      <td>
                        <span className={styles.rowActions}>
                          <ActionButton
                            type="button"
                            disabled={busy}
                            onClick={() => void openIdentityEdit(item.id)}
                          >
                            Edit
                          </ActionButton>
                          {item.status !== "archived" ? (
                            <ActionButton
                              type="button"
                              tone="dangerQuiet"
                              disabled={busy}
                              onClick={() => setArchiveId(item.id)}
                            >
                              Archive
                            </ActionButton>
                          ) : null}
                        </span>
                      </td>
                    </tr>
                    {open ? (
                      <tr className={styles.detailRow} id={detailId}>
                        <td colSpan={identityColumnCount}>
                          <div className={styles.detail}>
                            {rows === null ? (
                              <p className={styles.muted}>
                                {connections.error
                                  ? "Connections could not be loaded."
                                  : "Loading connections."}
                              </p>
                            ) : rows.length === 0 ? (
                              <p className={styles.muted}>No connections.</p>
                            ) : (
                              <div className={styles.tableWrap}>
                                <table>
                                  <caption>{`Connections for ${item.displayName}`}</caption>
                                  <thead>
                                    <tr>
                                      <th scope="col">Label</th>
                                      <th scope="col">Status</th>
                                      <th scope="col">Credentials</th>
                                      <th scope="col">Interval</th>
                                      <th scope="col">Freshness</th>
                                      <th scope="col">Secrets updated</th>
                                      <th scope="col">Actions</th>
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {rows.map((connection) => (
                                      <tr key={connection.id}>
                                        <td>{connection.label}</td>
                                        <td>{connection.status}</td>
                                        <td>
                                          {connection.credentialsConfigured
                                            ? "configured"
                                            : "missing"}
                                        </td>
                                        <td>
                                          {secondsLabel(connection.collectionIntervalSeconds)}
                                        </td>
                                        <td>{secondsLabel(connection.freshnessAfterSeconds)}</td>
                                        <td>{formatInstant(connection.credentialsUpdatedAt)}</td>
                                        <td>
                                          <span className={styles.rowActions}>
                                            <ActionButton
                                              type="button"
                                              disabled={busy}
                                              onClick={() =>
                                                void openConnectionEdit(
                                                  connection.id,
                                                  item.displayName,
                                                )
                                              }
                                            >
                                              Edit
                                            </ActionButton>
                                            <ActionButton
                                              type="button"
                                              tone="dangerQuiet"
                                              disabled={busy}
                                              onClick={() => setDeleteId(connection.id)}
                                            >
                                              Delete
                                            </ActionButton>
                                          </span>
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            )}
                            {item.status === "archived" ? null : (
                              <div className={styles.actions}>
                                <ActionButton
                                  type="button"
                                  tone="primary"
                                  disabled={busy}
                                  onClick={() => openConnectionCreate(item.id)}
                                >
                                  Add connection
                                </ActionButton>
                              </div>
                            )}
                          </div>
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                );
              })
            )}
          </tbody>
        </table>
      </div>
      <IdentityFormDialog
        portfolioId={portfolioId}
        request={identityRequest}
        identities={identityItems ?? []}
        onClose={() => setIdentityRequest(null)}
        onSaved={invalidateIdentities}
      />
      <ConnectionFormDialog
        portfolioId={portfolioId}
        request={connectionRequest}
        onClose={() => setConnectionRequest(null)}
        onSaved={invalidateConnections}
      />
      <Dialog open={Boolean(archiveId)} title="Archive identity" onClose={() => setArchiveId(null)}>
        <p className={styles.lead}>
          Archiving removes this identity from the current portfolio totals and from this list.
          History before the archive time stays on this identity. Disable every enabled connection
          first, and archive subordinate identities first. The same platform identity can be created
          again afterward.
        </p>
        <div className={styles.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={busy}
            onClick={() => void confirmArchive()}
          >
            Archive
          </ActionButton>
          <ActionButton type="button" onClick={() => setArchiveId(null)}>
            Cancel
          </ActionButton>
        </div>
      </Dialog>
      <Dialog open={Boolean(deleteId)} title="Delete connection" onClose={() => setDeleteId(null)}>
        <p className={styles.lead}>
          This removes the Connection and its ciphertext. Collection history on the Identity is
          kept.
        </p>
        <div className={styles.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={busy}
            onClick={() => void confirmDelete()}
          >
            Delete
          </ActionButton>
          <ActionButton type="button" onClick={() => setDeleteId(null)}>
            Cancel
          </ActionButton>
        </div>
      </Dialog>
    </section>
  );
}
