import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useState } from "react";
import { deleteIdentityCredentials, problemMessage, putIdentityCredentials } from "@/api";
import type { Identity, Platform } from "@/api";
import { credentialRequired } from "../lib/credentialRequirements";
import { CONFLICT } from "./identityCommon";
import { identityEtag } from "@/shared/lib/useIdentities";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextArea } from "@/shared/ui/FormControls";
import { PasswordInput } from "@/shared/ui/PasswordInput";
import styles from "@/features/manage/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function IdentityCredentialsForm({
  portfolioId,
  item,
  platform,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  item: Identity;
  platform: Platform;
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  const write = useAuthenticatedWrite();
  const [apiKey, setApiKey] = useState("");
  const [apiSecret, setApiSecret] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [futuresApiKey, setFuturesApiKey] = useState("");
  const [futuresApiSecret, setFuturesApiSecret] = useState("");
  const form = platform.credentialForm;
  const selectedTypes = item.accounts
    .filter((account) => account.selected)
    .map((account) => account.accountType);
  const spotRequired = credentialRequired(form.primary.requirement, selectedTypes);
  const futuresRequired = form.futures
    ? credentialRequired(form.futures.requirement, selectedTypes)
    : false;
  const passphraseRequired = form.passphrase === "required";
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (passphraseRequired && passphrase.trim() === "") {
      setError(`Passphrase is required for ${platform.name} keys.`);
      return;
    }
    if (
      form.futures &&
      (!apiKey.trim() !== !apiSecret.trim() ||
        !futuresApiKey.trim() !== !futuresApiSecret.trim() ||
        (!apiKey.trim() && !futuresApiKey.trim()))
    ) {
      setError("Provide a complete API key and secret for at least one credential group.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        putIdentityCredentials({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
          body: {
            apiKey: apiKey.trim(),
            apiSecret: apiSecret.trim(),
            ...(passphrase.trim() ? { passphrase: passphrase.trim() } : {}),
            ...(form.futures
              ? { futuresApiKey: futuresApiKey.trim(), futuresApiSecret: futuresApiSecret.trim() }
              : {}),
          },
        }),
      );
      if (result.response?.status === 412) {
        await onSaved(CONFLICT);
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The credentials could not be stored."));
        return;
      }
      setApiKey("");
      setApiSecret("");
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
      const result = await write(() =>
        deleteIdentityCredentials({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        }),
      );
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

  const primaryFields = (
    <>
      <label className={shared.field}>
        <span>
          {form.primary.keyLabel}
          {form.futures ? ` (${spotRequired ? "required" : "optional"})` : null}
        </span>
        <PasswordInput
          fieldName={form.primary.keyLabel}
          autoComplete="off"
          value={apiKey}
          onChange={(event) => setApiKey(event.target.value)}
          required={spotRequired || apiSecret.trim() !== ""}
        />
      </label>
      <label className={shared.field}>
        <span>
          {form.primary.secretLabel}
          {form.futures ? ` (${spotRequired ? "required" : "optional"})` : null}
        </span>
        {form.primary.secretInput === "multiline" ? (
          <TextArea
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
            rows={6}
            value={apiSecret}
            onChange={(event) => setApiSecret(event.target.value)}
            required={spotRequired || apiKey.trim() !== ""}
          />
        ) : (
          <PasswordInput
            fieldName={form.primary.secretLabel}
            autoComplete="off"
            value={apiSecret}
            onChange={(event) => setApiSecret(event.target.value)}
            required={spotRequired || apiKey.trim() !== ""}
          />
        )}
      </label>
    </>
  );

  return (
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)} autoComplete="off">
      <p className={shared.notice}>
        Read-only keys only. Credentials are encrypted and write-only; existing keys cannot be
        viewed.
        {item.connection.credentialsConfigured
          ? " Saving replaces stored keys."
          : " No credentials stored yet."}
      </p>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      {form.futures ? (
        <div className={styles.credentialsGroup}>
          <span className={styles.credentialsGroupTitle}>
            {form.primary.keyLabel.includes("Spot") ? "Spot credentials" : "Primary credentials"}
          </span>
          {primaryFields}
        </div>
      ) : (
        primaryFields
      )}
      {form.help ? <p className={shared.muted}>{form.help}</p> : null}
      {form.futures ? (
        <div className={styles.credentialsGroup}>
          <span className={styles.credentialsGroupTitle}>Futures credentials</span>
          <label className={shared.field}>
            <span>
              {form.futures.keyLabel} ({futuresRequired ? "required" : "optional"})
            </span>
            <PasswordInput
              fieldName={form.futures.keyLabel}
              autoComplete="off"
              value={futuresApiKey}
              onChange={(event) => setFuturesApiKey(event.target.value)}
              required={futuresRequired || futuresApiSecret.trim() !== ""}
            />
          </label>
          <label className={shared.field}>
            <span>
              {form.futures.secretLabel} ({futuresRequired ? "required" : "optional"})
            </span>
            <PasswordInput
              fieldName={form.futures.secretLabel}
              autoComplete="off"
              value={futuresApiSecret}
              onChange={(event) => setFuturesApiSecret(event.target.value)}
              required={futuresRequired || futuresApiKey.trim() !== ""}
            />
          </label>
        </div>
      ) : null}
      {form.passphrase !== "hidden" ? (
        <label className={shared.field}>
          <span>{passphraseRequired ? "Passphrase" : "Passphrase (optional)"}</span>
          <PasswordInput
            fieldName="Passphrase"
            autoComplete="off"
            value={passphrase}
            onChange={(event) => setPassphrase(event.target.value)}
            required={passphraseRequired}
          />
        </label>
      ) : null}
      {confirmRemove ? (
        <div className={styles.credentialsConfirmCard}>
          <span className={styles.credentialsConfirmText}>
            Remove credentials? Add new ones and enable the connection to resume.
          </span>
          <div className={shared.dialogActions}>
            <ActionButton
              type="button"
              tone="ghost"
              disabled={busy}
              onClick={() => setConfirmRemove(false)}
            >
              Cancel
            </ActionButton>
            <ActionButton type="button" tone="danger" disabled={busy} onClick={() => void remove()}>
              Confirm removal
            </ActionButton>
          </div>
        </div>
      ) : (
        <div className={shared.dialogActions}>
          {item.connection.credentialsConfigured ? (
            <span className={shared.dialogActionLead}>
              <ActionButton
                type="button"
                tone="dangerQuiet"
                disabled={busy}
                onClick={() => setConfirmRemove(true)}
              >
                Remove credentials
              </ActionButton>
            </span>
          ) : null}
          <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
            Cancel
          </ActionButton>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Store credentials
          </ActionButton>
        </div>
      )}
    </form>
  );
}
