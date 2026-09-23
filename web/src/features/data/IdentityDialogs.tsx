import { type FormEvent, useRef, useState } from "react";
import {
  createIdentity,
  deleteIdentityCredentials,
  patchIdentity,
  putIdentityCredentials,
} from "@/api/generated/sdk.gen";
import type { Identity, Platform } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { identityEtag } from "@/features/data/useIdentities";
import { newIdempotencyKey } from "@/shared/lib/id";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextInput, Toggle } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const CONFLICT = "The identity changed in another session. The list was reloaded; open it again.";

export type IdentityDialogInput =
  { kind: "create" } | { kind: "edit"; item: Identity } | { kind: "credentials"; item: Identity };

export type IdentityDialogRequest = IdentityDialogInput & { nonce: number };

interface DialogProps {
  portfolioId: string;
  request: IdentityDialogRequest | null;
  platforms: Platform[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}

export function IdentityDialog({ portfolioId, request, platforms, onClose, onSaved }: DialogProps) {
  const title =
    request?.kind === "edit"
      ? `Edit ${request.item.displayName}`
      : request?.kind === "credentials"
        ? `Credentials for ${request.item.displayName}`
        : "New identity";
  return (
    <Dialog open={request !== null} title={title} onClose={onClose}>
      {request?.kind === "create" ? (
        <CreateForm
          key={request.nonce}
          portfolioId={portfolioId}
          platforms={platforms}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "edit" ? (
        <EditForm
          key={request.nonce}
          portfolioId={portfolioId}
          item={request.item}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "credentials" ? (
        <CredentialsForm
          key={request.nonce}
          portfolioId={portfolioId}
          item={request.item}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
    </Dialog>
  );
}

function CreateForm({
  portfolioId,
  platforms,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  platforms: Platform[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  const { redirect } = useSessionScope();
  const createKey = useRef(newIdempotencyKey());
  const firstPlatform = platforms[0];
  const [platformId, setPlatformId] = useState(firstPlatform?.id ?? "");
  const [modeId, setModeId] = useState(firstPlatform?.accountModes[0]?.id ?? "");
  const [selectedTypes, setSelectedTypes] = useState<string[]>(
    () => firstPlatform?.accountModes[0]?.accountTypes.map((type) => type.id) ?? [],
  );
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const platform = platforms.find((item) => item.id === platformId);
  const mode = platform?.accountModes.find((item) => item.id === modeId);

  function choosePlatform(nextId: string) {
    const next = platforms.find((item) => item.id === nextId);
    const firstMode = next?.accountModes[0];
    setPlatformId(nextId);
    setModeId(firstMode?.id ?? "");
    setSelectedTypes(firstMode?.accountTypes.map((type) => type.id) ?? []);
  }

  function chooseMode(nextId: string) {
    const next = platform?.accountModes.find((item) => item.id === nextId);
    setModeId(nextId);
    setSelectedTypes(next?.accountTypes.map((type) => type.id) ?? []);
  }

  function toggleType(typeId: string, checked: boolean) {
    setSelectedTypes((current) =>
      checked ? [...new Set([...current, typeId])] : current.filter((id) => id !== typeId),
    );
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (selectedTypes.length === 0) {
      setError("Select at least one account type; only selected accounts join the boundary.");
      return;
    }
    if (displayName.trim() === "") {
      setError("Display name is required.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await createIdentity({
        path: { portfolioId },
        headers: { "Idempotency-Key": createKey.current },
        body: {
          platform: platformId,
          displayName: displayName.trim(),
          accountMode: modeId,
          selectedAccountTypes: selectedTypes,
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
      await onSaved(
        `Identity ${result.data.displayName} created. Store its credentials, then enable the connection.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be created."));
    } finally {
      setBusy(false);
    }
  }

  if (platforms.length === 0) {
    return <p className={styles.notice}>No Platforms are available on this server.</p>;
  }

  return (
    <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={styles.lead}>
        One Identity is one Platform login. Pick its Account Mode and the Account Types to include;
        only selected accounts count toward the Portfolio.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={styles.field}>
        <span>Platform</span>
        <Select
          value={platformId}
          onChange={(event) => choosePlatform(event.target.value)}
          required
        >
          {platforms.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
              {item.adapterAvailable ? "" : " (adapter unavailable)"}
            </option>
          ))}
        </Select>
      </label>
      {platform && !platform.adapterAvailable ? (
        <p className={styles.notice}>
          This server has no adapter for {platform.name}; the identity can be created but not
          collected.
        </p>
      ) : null}
      <label className={styles.field}>
        <span>Account mode</span>
        <Select value={modeId} onChange={(event) => chooseMode(event.target.value)} required>
          {(platform?.accountModes ?? []).map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </Select>
      </label>
      <fieldset className={styles.field}>
        <legend>
          <span>Account types</span>
        </legend>
        <div className={styles.checkList}>
          {(mode?.accountTypes ?? []).map((type) => (
            <Toggle
              key={type.id}
              checked={selectedTypes.includes(type.id)}
              onCheckedChange={(checked) => toggleType(type.id, checked)}
            >
              {type.name}
            </Toggle>
          ))}
        </div>
      </fieldset>
      <label className={styles.field}>
        <span>Display name</span>
        <TextInput
          value={displayName}
          onChange={(event) => setDisplayName(event.target.value)}
          maxLength={120}
          required
        />
      </label>
      <div className={styles.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          Create identity
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
    </form>
  );
}

function EditForm({
  portfolioId,
  item,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  item: Identity;
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  const { redirect } = useSessionScope();
  const [displayName, setDisplayName] = useState(item.displayName);
  const [selected, setSelected] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(item.accounts.map((account) => [account.id, account.selected])),
  );
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (displayName.trim() === "") {
      setError("Display name is required.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await patchIdentity({
        path: { portfolioId, identityId: item.id },
        headers: { "If-Match": identityEtag(item) },
        body: {
          displayName: displayName.trim(),
          accounts: item.accounts.map((account) => ({
            id: account.id,
            selected: selected[account.id] ?? account.selected,
          })),
        },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onSaved(CONFLICT);
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The identity could not be updated."));
        return;
      }
      await onSaved(`Identity ${result.data.displayName} updated.`);
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be updated."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
      <p className={styles.lead}>
        Deselecting an account removes it from the boundary at the next Collection Cycle, which
        records a Boundary Event; selecting one adds it back.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={styles.field}>
        <span>Display name</span>
        <TextInput
          value={displayName}
          onChange={(event) => setDisplayName(event.target.value)}
          maxLength={120}
          required
        />
      </label>
      <fieldset className={styles.field}>
        <legend>
          <span>Selected accounts</span>
        </legend>
        <div className={styles.checkList}>
          {item.accounts.map((account) => (
            <Toggle
              key={account.id}
              checked={selected[account.id] ?? account.selected}
              onCheckedChange={(checked) =>
                setSelected((current) => ({ ...current, [account.id]: checked }))
              }
            >
              {account.displayName}
            </Toggle>
          ))}
        </div>
      </fieldset>
      <div className={styles.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          Save identity
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
    </form>
  );
}

function CredentialsForm({
  portfolioId,
  item,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  item: Identity;
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  const { redirect } = useSessionScope();
  const [apiKey, setApiKey] = useState("");
  const [apiSecret, setApiSecret] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await putIdentityCredentials({
        path: { portfolioId, identityId: item.id },
        headers: { "If-Match": identityEtag(item) },
        body: {
          apiKey: apiKey.trim(),
          apiSecret: apiSecret.trim(),
          ...(passphrase.trim() ? { passphrase: passphrase.trim() } : {}),
        },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onSaved(CONFLICT);
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The credentials could not be stored."));
        return;
      }
      await onSaved(
        `Credentials stored for ${result.data.displayName}. The connection is ${result.data.connection.status}; validate or enable it next.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The credentials could not be stored."));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await deleteIdentityCredentials({
        path: { portfolioId, identityId: item.id },
        headers: { "If-Match": identityEtag(item) },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onSaved(CONFLICT);
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The credentials could not be removed."));
        return;
      }
      await onSaved(
        `Credentials removed from ${result.data.displayName}. The connection is ${result.data.connection.status}.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The credentials could not be removed."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className={styles.form} onSubmit={(event) => void onSubmit(event)} autoComplete="off">
      <p className={styles.lead}>
        Read-only API keys only. Credentials are write-only: the server never returns them.
        {item.connection.credentialsConfigured
          ? " Saving replaces the stored credentials."
          : " None are stored yet."}
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={styles.field}>
        <span>API key</span>
        <TextInput
          type="password"
          autoComplete="off"
          value={apiKey}
          onChange={(event) => setApiKey(event.target.value)}
          required
        />
      </label>
      <label className={styles.field}>
        <span>API secret</span>
        <TextInput
          type="password"
          autoComplete="off"
          value={apiSecret}
          onChange={(event) => setApiSecret(event.target.value)}
          required
        />
      </label>
      <label className={styles.field}>
        <span>Passphrase (optional)</span>
        <TextInput
          type="password"
          autoComplete="off"
          value={passphrase}
          onChange={(event) => setPassphrase(event.target.value)}
        />
      </label>
      <div className={styles.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          Store credentials
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
      {item.connection.credentialsConfigured ? (
        <div className={styles.actions}>
          {confirmRemove ? (
            <>
              <span className={styles.muted}>
                Removing disables the connection until new credentials are stored.
              </span>
              <ActionButton
                type="button"
                tone="danger"
                disabled={busy}
                onClick={() => void remove()}
              >
                Confirm removal
              </ActionButton>
              <ActionButton
                type="button"
                tone="ghost"
                disabled={busy}
                onClick={() => setConfirmRemove(false)}
              >
                Keep
              </ActionButton>
            </>
          ) : (
            <ActionButton
              type="button"
              tone="dangerQuiet"
              disabled={busy}
              onClick={() => setConfirmRemove(true)}
            >
              Remove credentials
            </ActionButton>
          )}
        </div>
      ) : null}
    </form>
  );
}
