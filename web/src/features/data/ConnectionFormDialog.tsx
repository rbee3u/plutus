import { type FormEvent, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { createConnection, getConnection, patchConnection } from "@/api/generated/sdk.gen";
import type { Connection, ConnectionCredentials } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { newIdempotencyKey } from "@/shared/lib/id";
import { readEtag } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

interface ConnectionForm {
  label: string;
  status: "enabled" | "disabled" | "invalid";
  collectionIntervalSeconds: string;
  freshnessAfterSeconds: string;
  apiKey: string;
  apiSecret: string;
  passphrase: string;
}

const emptyForm: ConnectionForm = {
  label: "",
  status: "disabled",
  collectionIntervalSeconds: "",
  freshnessAfterSeconds: "",
  apiKey: "",
  apiSecret: "",
  passphrase: "",
};

export type ConnectionDialogRequest =
  | { nonce: number; mode: "create"; identityId: string; identityName: string }
  | {
      nonce: number;
      mode: "edit";
      item: Connection;
      etag: string;
      identityName: string;
    };

function formFromConnection(item: Connection): ConnectionForm {
  return {
    label: item.label,
    status: item.status,
    collectionIntervalSeconds:
      item.collectionIntervalSeconds != null ? String(item.collectionIntervalSeconds) : "",
    freshnessAfterSeconds:
      item.freshnessAfterSeconds != null ? String(item.freshnessAfterSeconds) : "",
    apiKey: "",
    apiSecret: "",
    passphrase: "",
  };
}

export function ConnectionFormDialog({
  portfolioId,
  request,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  request: ConnectionDialogRequest | null;
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  return (
    <Dialog
      open={request !== null}
      title={request?.mode === "edit" ? "Edit connection" : "Create connection"}
      onClose={onClose}
    >
      {request ? (
        <ConnectionFormBody
          key={request.nonce}
          portfolioId={portfolioId}
          request={request}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
    </Dialog>
  );
}

function ConnectionFormBody({
  portfolioId,
  request,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  request: ConnectionDialogRequest;
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const createKey = useRef(newIdempotencyKey());
  const [form, setForm] = useState<ConnectionForm>(() =>
    request.mode === "edit" ? formFromConnection(request.item) : emptyForm,
  );
  const [editingId, setEditingId] = useState<string | null>(
    request.mode === "edit" ? request.item.id : null,
  );
  const [etag, setEtag] = useState(request.mode === "edit" ? request.etag : "");
  const [configured, setConfigured] = useState(
    request.mode === "edit" ? request.item.credentialsConfigured : false,
  );
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const credentialsFilled = Boolean(form.apiKey.trim() && form.apiSecret.trim());
  const canEnable = configured || credentialsFilled;
  const boundIdentityId = request.mode === "create" ? request.identityId : null;

  function applyConnection(item: Connection, nextEtag: string) {
    setEditingId(item.id);
    setEtag(nextEtag);
    setConfigured(item.credentialsConfigured);
    setForm(formFromConnection(item));
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
      } else if (boundIdentityId) {
        const result = await createConnection({
          path: { portfolioId },
          headers: { "Idempotency-Key": createKey.current },
          body: {
            identityId: boundIdentityId,
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
        createKey.current = newIdempotencyKey();
        setForm(emptyForm);
        setNotice("Connection created. Secrets are write-only and were not returned.");
      } else {
        return;
      }
      await onSaved();
    } catch (cause) {
      setError(problemMessage(cause, "The connection could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <p className={styles.lead}>
        Credential fields are write-only; the API only reports whether ciphertext exists.
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
          <TextInput value={request.identityName} disabled />
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
          <ActionButton type="button" onClick={onClose}>
            Cancel
          </ActionButton>
        </div>
      </form>
    </>
  );
}
