import { type FormEvent, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { createIdentity, getIdentity, patchIdentity } from "@/api/generated/sdk.gen";
import type { Identity } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { modesForPlatform, PLATFORMS, type AccountMode } from "@/features/data/platforms";
import { newIdempotencyKey } from "@/shared/lib/id";
import { readEtag } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

interface IdentityForm {
  platformId: string;
  kind: "main" | "subordinate";
  parentIdentityId: string;
  externalRef: string;
  displayName: string;
  accountMode: AccountMode | "";
  status: "enabled" | "disabled";
}

const emptyForm: IdentityForm = {
  platformId: PLATFORMS[0].id,
  kind: "main",
  parentIdentityId: "",
  externalRef: "",
  displayName: "",
  accountMode: PLATFORMS[0].modes[0],
  status: "enabled",
};

export type IdentityDialogRequest =
  | { nonce: number; mode: "create" }
  | { nonce: number; mode: "edit"; item: Identity; etag: string; frozen: boolean };

function formFromIdentity(item: Identity): IdentityForm {
  return {
    platformId: item.platformId,
    kind: item.kind,
    parentIdentityId: item.parentIdentityId ?? "",
    externalRef: item.externalRef,
    displayName: item.displayName,
    accountMode: item.accountMode,
    status: item.status === "enabled" ? "enabled" : "disabled",
  };
}

export function IdentityFormDialog({
  portfolioId,
  request,
  identities,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  request: IdentityDialogRequest | null;
  identities: Identity[];
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  return (
    <Dialog
      open={request !== null}
      title={request?.mode === "edit" ? "Edit identity" : "Create identity"}
      onClose={onClose}
    >
      {request ? (
        <IdentityForm
          key={request.nonce}
          portfolioId={portfolioId}
          request={request}
          identities={identities}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
    </Dialog>
  );
}

function IdentityForm({
  portfolioId,
  request,
  identities,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  request: IdentityDialogRequest;
  identities: Identity[];
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const createKey = useRef(newIdempotencyKey());
  const [form, setForm] = useState<IdentityForm>(() =>
    request.mode === "edit" ? formFromIdentity(request.item) : emptyForm,
  );
  const [editingId, setEditingId] = useState<string | null>(
    request.mode === "edit" ? request.item.id : null,
  );
  const [etag, setEtag] = useState(request.mode === "edit" ? request.etag : "");
  const [frozen, setFrozen] = useState(request.mode === "edit" ? request.frozen : false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const mains = identities.filter((item) => item.kind === "main" && item.status !== "archived");
  const modes = modesForPlatform(form.platformId);

  function applyIdentity(item: Identity, nextEtag: string, nextFrozen: boolean) {
    setEditingId(item.id);
    setEtag(nextEtag);
    setFrozen(nextFrozen);
    setForm(formFromIdentity(item));
  }

  async function loadLatest(identityId: string, message: string) {
    const result = await getIdentity({ path: { portfolioId, identityId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      applyIdentity(
        result.data,
        readEtag(result.response, `"identity:${String(result.data.version)}"`),
        frozen,
      );
    }
    setError(message);
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (request.mode === "edit" && request.item.status === "archived") {
      setError("Archived identities cannot be changed.");
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      if (editingId) {
        const result = await patchIdentity({
          path: { portfolioId, identityId: editingId },
          headers: { "If-Match": etag },
          body: {
            displayName: form.displayName,
            status: form.status,
            ...(frozen
              ? {}
              : {
                  externalRef: form.externalRef,
                  accountMode: form.accountMode || undefined,
                  parentIdentityId:
                    form.kind === "subordinate" ? form.parentIdentityId || null : null,
                }),
          },
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.response?.status === 412) {
          await loadLatest(
            editingId,
            "The identity changed in another session. The form now shows the latest server values.",
          );
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The identity could not be updated."));
          return;
        }
        applyIdentity(
          result.data,
          readEtag(result.response, `"identity:${String(result.data.version)}"`),
          frozen,
        );
        setNotice("Identity updated.");
      } else {
        const result = await createIdentity({
          path: { portfolioId },
          headers: { "Idempotency-Key": createKey.current },
          body: {
            platformId: form.platformId,
            displayName: form.displayName,
            externalRef: form.externalRef,
            accountMode: form.accountMode || PLATFORMS[0].modes[0],
            kind: form.kind,
            parentIdentityId: form.kind === "subordinate" ? form.parentIdentityId : null,
          },
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The identity could not be created."));
          return;
        }
        createKey.current = newIdempotencyKey();
        setForm({ ...emptyForm, platformId: form.platformId, accountMode: form.accountMode });
        setNotice("Identity created.");
      }
      await onSaved();
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
        <label className={styles.field}>
          <span>Platform</span>
          <Select
            value={form.platformId}
            disabled={Boolean(editingId)}
            onChange={(event) => {
              const platformId = event.target.value;
              const nextModes = modesForPlatform(platformId);
              setForm((current) => ({
                ...current,
                platformId,
                accountMode: nextModes[0] ?? "",
              }));
            }}
            required
          >
            {PLATFORMS.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Kind</span>
          <Select
            value={form.kind}
            disabled={Boolean(editingId)}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                kind: event.target.value as IdentityForm["kind"],
                parentIdentityId: event.target.value === "main" ? "" : current.parentIdentityId,
              }))
            }
            required
          >
            <option value="main">Main</option>
            <option value="subordinate">Subordinate</option>
          </Select>
        </label>
        {form.kind === "subordinate" ? (
          <label className={styles.field}>
            <span>Parent identity</span>
            <Select
              value={form.parentIdentityId}
              disabled={Boolean(editingId) && frozen}
              onChange={(event) =>
                setForm((current) => ({ ...current, parentIdentityId: event.target.value }))
              }
              required
            >
              <option value="" disabled>
                Select a main identity
              </option>
              {mains.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.displayName}
                </option>
              ))}
            </Select>
          </label>
        ) : null}
        <label className={styles.field}>
          <span>Display name</span>
          <TextInput
            value={form.displayName}
            onChange={(event) =>
              setForm((current) => ({ ...current, displayName: event.target.value }))
            }
            required
          />
        </label>
        <label className={styles.field}>
          <span>External ref</span>
          <TextInput
            value={form.externalRef}
            disabled={Boolean(editingId) && frozen}
            onChange={(event) =>
              setForm((current) => ({ ...current, externalRef: event.target.value }))
            }
            required
          />
        </label>
        <label className={styles.field}>
          <span>Account mode</span>
          <Select
            value={form.accountMode}
            disabled={Boolean(editingId) && frozen}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                accountMode: event.target.value as AccountMode,
              }))
            }
            required
          >
            {modes.map((mode) => (
              <option key={mode} value={mode}>
                {mode}
              </option>
            ))}
          </Select>
        </label>
        {editingId ? (
          <label className={styles.field}>
            <span>Status</span>
            <Select
              value={form.status}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  status: event.target.value as IdentityForm["status"],
                }))
              }
            >
              <option value="enabled">Enabled</option>
              <option value="disabled">Disabled</option>
            </Select>
          </label>
        ) : null}
        {frozen ? (
          <p className={styles.muted}>
            Observations exist for this identity, so platform fields can no longer change.
          </p>
        ) : null}
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            {editingId ? "Save identity" : "Create identity"}
          </ActionButton>
          <ActionButton type="button" onClick={onClose}>
            Cancel
          </ActionButton>
        </div>
      </form>
    </>
  );
}
