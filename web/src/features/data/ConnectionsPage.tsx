import { type FormEvent, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createConnection,
  deleteConnection,
  getConnection,
  listConnections,
  listIdentities,
  patchConnection,
} from "@/api/generated/sdk.gen";
import type { Connection, ConnectionCredentials } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { formatInstant } from "@/shared/lib/datetime";
import { readEtag, unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

interface ConnectionForm {
  identityId: string;
  label: string;
  status: "enabled" | "disabled" | "invalid";
  collectionIntervalSeconds: string;
  freshnessAfterSeconds: string;
  apiKey: string;
  apiSecret: string;
  passphrase: string;
}

const emptyForm: ConnectionForm = {
  identityId: "",
  label: "",
  status: "disabled",
  collectionIntervalSeconds: "",
  freshnessAfterSeconds: "",
  apiKey: "",
  apiSecret: "",
  passphrase: "",
};

export function ConnectionsPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const createKey = useRef(crypto.randomUUID());
  const [form, setForm] = useState<ConnectionForm>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [etag, setEtag] = useState("");
  const [configured, setConfigured] = useState(false);
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
  const identityName = (id: string) =>
    identities.data?.items.find((item) => item.id === id)?.displayName ?? id;
  const credentialsFilled = Boolean(form.apiKey.trim() && form.apiSecret.trim());
  const canEnable = configured || credentialsFilled;

  function applyConnection(item: Connection, nextEtag: string) {
    setEditingId(item.id);
    setEtag(nextEtag);
    setConfigured(item.credentialsConfigured);
    setForm({
      identityId: item.identityId,
      label: item.label,
      status: item.status,
      collectionIntervalSeconds:
        item.collectionIntervalSeconds != null ? String(item.collectionIntervalSeconds) : "",
      freshnessAfterSeconds:
        item.freshnessAfterSeconds != null ? String(item.freshnessAfterSeconds) : "",
      apiKey: "",
      apiSecret: "",
      passphrase: "",
    });
  }

  async function startEdit(connectionId: string) {
    setError(null);
    setNotice(null);
    const result = await getConnection({ path: { portfolioId, connectionId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    const item = unwrapOrRedirect(result, redirect);
    applyConnection(item, readEtag(result.response, `"connection:${String(item.version)}"`));
  }

  async function loadLatest(connectionId: string, message: string) {
    const result = await getConnection({ path: { portfolioId, connectionId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      applyConnection(
        result.data,
        readEtag(result.response, `"connection:${String(result.data.version)}"`),
      );
    }
    setError(message);
  }

  async function invalidate() {
    await queryClient.invalidateQueries({ queryKey: ["connections", user.data?.id, portfolioId] });
  }

  function credentialsBody(): ConnectionCredentials | undefined {
    if (!form.apiKey.trim() && !form.apiSecret.trim() && !form.passphrase.trim()) {
      return undefined;
    }
    return {
      apiKey: form.apiKey,
      apiSecret: form.apiSecret,
      passphrase: form.passphrase.trim() ? form.passphrase : undefined,
    };
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (form.status === "enabled" && !canEnable) {
      setError("Store credentials before enabling this Connection.");
      return;
    }
    if (
      (form.apiKey.trim() || form.apiSecret.trim() || form.passphrase.trim()) &&
      (!form.apiKey.trim() || !form.apiSecret.trim())
    ) {
      setError(
        "API key and secret are required together. Leave both blank to keep stored secrets.",
      );
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const credentials = credentialsBody();
      const interval = form.collectionIntervalSeconds
        ? Number(form.collectionIntervalSeconds)
        : undefined;
      const freshness = form.freshnessAfterSeconds ? Number(form.freshnessAfterSeconds) : undefined;
      if (editingId) {
        const result = await patchConnection({
          path: { portfolioId, connectionId: editingId },
          headers: { "If-Match": etag },
          body: {
            label: form.label,
            status: form.status,
            collectionIntervalSeconds: interval,
            freshnessAfterSeconds: freshness,
            credentials,
          },
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.response?.status === 412) {
          await loadLatest(
            editingId,
            "The connection changed in another session. The form now shows the latest server values.",
          );
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The connection could not be updated."));
          return;
        }
        applyConnection(
          result.data,
          readEtag(result.response, `"connection:${String(result.data.version)}"`),
        );
        setNotice("Connection updated. Secrets are write-only and were not returned.");
      } else {
        const result = await createConnection({
          path: { portfolioId },
          headers: { "Idempotency-Key": createKey.current },
          body: {
            identityId: form.identityId,
            label: form.label,
            status: form.status,
            collectionIntervalSeconds: interval,
            freshnessAfterSeconds: freshness,
            credentials,
          },
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The connection could not be created."));
          return;
        }
        createKey.current = crypto.randomUUID();
        setForm(emptyForm);
        setNotice("Connection created. Secrets are write-only and were not returned.");
      }
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The connection could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  async function confirmDelete() {
    if (!deleteId) {
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const current = await getConnection({ path: { portfolioId, connectionId: deleteId } });
      if (current.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(current, redirect);
      const match = readEtag(current.response, `"connection:${String(item.version)}"`);
      await refreshCsrf();
      const result = await deleteConnection({
        path: { portfolioId, connectionId: deleteId },
        headers: { "If-Match": match },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setDeleteId(null);
        await loadLatest(
          deleteId,
          "The connection changed in another session. Reload before deleting.",
        );
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The connection could not be deleted."));
        return;
      }
      if (editingId === deleteId) {
        setEditingId(null);
        setForm(emptyForm);
        setEtag("");
        setConfigured(false);
      }
      setDeleteId(null);
      setNotice("Connection deleted.");
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The connection could not be deleted."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.wide}>
      <h2>Connections</h2>
      <p className={styles.lead}>
        Collection interval and freshness windows are stored here. Credential fields are write-only;
        the API only reports whether ciphertext exists.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)} autoComplete="off">
        <label className={styles.field}>
          <span>Identity</span>
          <Select
            value={form.identityId}
            disabled={Boolean(editingId)}
            onChange={(event) =>
              setForm((current) => ({ ...current, identityId: event.target.value }))
            }
            required
          >
            <option value="" disabled>
              Select an identity
            </option>
            {(identities.data?.items ?? [])
              .filter((item) => item.status !== "archived")
              .map((item) => (
                <option key={item.id} value={item.id}>
                  {item.displayName}
                </option>
              ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Label</span>
          <TextInput
            value={form.label}
            onChange={(event) => setForm((current) => ({ ...current, label: event.target.value }))}
            required
          />
        </label>
        <label className={styles.field}>
          <span>Status</span>
          <Select
            value={form.status}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                status: event.target.value as ConnectionForm["status"],
              }))
            }
          >
            <option value="disabled">Disabled</option>
            <option value="enabled" disabled={!canEnable}>
              Enabled
            </option>
            <option value="invalid">Invalid</option>
          </Select>
        </label>
        <label className={styles.field}>
          <span>Collection interval (seconds)</span>
          <TextInput
            type="number"
            min={1}
            value={form.collectionIntervalSeconds}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                collectionIntervalSeconds: event.target.value,
              }))
            }
          />
        </label>
        <label className={styles.field}>
          <span>Freshness after (seconds)</span>
          <TextInput
            type="number"
            min={1}
            value={form.freshnessAfterSeconds}
            onChange={(event) =>
              setForm((current) => ({ ...current, freshnessAfterSeconds: event.target.value }))
            }
          />
        </label>
        <label className={styles.field}>
          <span>API key</span>
          <TextInput
            type="password"
            autoComplete="off"
            value={form.apiKey}
            placeholder={
              editingId && configured ? "Leave blank to keep the stored secret" : undefined
            }
            onChange={(event) => setForm((current) => ({ ...current, apiKey: event.target.value }))}
          />
        </label>
        <label className={styles.field}>
          <span>API secret</span>
          <TextInput
            type="password"
            autoComplete="off"
            value={form.apiSecret}
            placeholder={
              editingId && configured ? "Leave blank to keep the stored secret" : undefined
            }
            onChange={(event) =>
              setForm((current) => ({ ...current, apiSecret: event.target.value }))
            }
          />
        </label>
        <label className={styles.field}>
          <span>Passphrase (optional)</span>
          <TextInput
            type="password"
            autoComplete="off"
            value={form.passphrase}
            placeholder={
              editingId && configured ? "Leave blank to keep the stored secret" : undefined
            }
            onChange={(event) =>
              setForm((current) => ({ ...current, passphrase: event.target.value }))
            }
          />
        </label>
        {editingId ? (
          <p className={styles.muted}>
            Credentials configured: {configured ? "yes" : "no"}. Validation and identity refresh
            stay unimplemented in this phase.
          </p>
        ) : null}
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            {editingId ? "Save connection" : "Create connection"}
          </ActionButton>
          {editingId ? (
            <ActionButton
              type="button"
              onClick={() => {
                setEditingId(null);
                setForm(emptyForm);
                setEtag("");
                setConfigured(false);
              }}
            >
              Cancel edit
            </ActionButton>
          ) : null}
        </div>
      </form>
      <DataTable
        caption="Portfolio connections"
        columns={[
          { key: "label", header: "Label" },
          { key: "identity", header: "Identity" },
          { key: "status", header: "Status" },
          { key: "credentials", header: "Credentials" },
          { key: "interval", header: "Interval" },
          { key: "freshness", header: "Freshness" },
          { key: "updated", header: "Secrets updated" },
        ]}
        rows={(connections.data?.items ?? []).map((item) => ({
          id: item.id,
          label: item.label,
          identity: identityName(item.identityId),
          status: item.status,
          credentials: item.credentialsConfigured ? "configured" : "missing",
          interval:
            item.collectionIntervalSeconds != null
              ? `${String(item.collectionIntervalSeconds)}s`
              : "—",
          freshness:
            item.freshnessAfterSeconds != null ? `${String(item.freshnessAfterSeconds)}s` : "—",
          updated: formatInstant(item.credentialsUpdatedAt),
        }))}
      />
      <ul className={styles.flowActions}>
        {(connections.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>{item.label}</span>
            <span className={styles.actions}>
              <ActionButton type="button" disabled={busy} onClick={() => void startEdit(item.id)}>
                Edit
              </ActionButton>
              <ActionButton
                type="button"
                tone="dangerQuiet"
                disabled={busy}
                onClick={() => setDeleteId(item.id)}
              >
                Delete
              </ActionButton>
            </span>
          </li>
        ))}
      </ul>
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
