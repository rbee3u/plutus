import { type FormEvent, useState } from "react";
import {
  deleteIdentityCredentials,
  problemMessage,
  putIdentityCredentials,
  refreshCsrf,
} from "@/api";
import type { Identity } from "@/api";
import { CONFLICT } from "./identityCommon";
import { identityEtag } from "@/shared/lib/useIdentities";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextArea } from "@/shared/ui/FormControls";
import { PasswordInput } from "@/shared/ui/PasswordInput";
import shared from "@/shared/styles/page.module.css";

export function CredentialsForm({
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
  const [futuresApiKey, setFuturesApiKey] = useState("");
  const [futuresApiSecret, setFuturesApiSecret] = useState("");
  const kraken = item.platform === "kraken";
  const binance = item.platform === "binance";
  const selectedTypes = item.accounts
    .filter((account) => account.selected)
    .map((account) => account.accountType);
  const spotRequired = !kraken || selectedTypes.some((type) => type === "main" || type === "earn");
  const futuresRequired = kraken && selectedTypes.some((type) => type !== "earn");
  // OKX signs every request with the passphrase chosen when the key was created.
  const passphraseRequired = item.platform === "okx";
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (passphraseRequired && passphrase.trim() === "") {
      setError("Passphrase is required for OKX keys.");
      return;
    }
    if (
      kraken &&
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
      await refreshCsrf();
      const result = await putIdentityCredentials({
        path: { portfolioId, identityId: item.id },
        headers: { "If-Match": identityEtag(item) },
        body: {
          apiKey: apiKey.trim(),
          apiSecret: apiSecret.trim(),
          ...(passphrase.trim() ? { passphrase: passphrase.trim() } : {}),
          ...(kraken
            ? { futuresApiKey: futuresApiKey.trim(), futuresApiSecret: futuresApiSecret.trim() }
            : {}),
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
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)} autoComplete="off">
      <p className={shared.notice}>
        Read-only API keys only. Credentials are write-only: the server never returns them.
        {item.connection.credentialsConfigured
          ? " Saving replaces the stored credentials."
          : " None are stored yet."}
      </p>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={shared.field}>
        <span>
          {kraken ? `Spot API key (${spotRequired ? "required" : "optional"})` : "API key"}
        </span>
        <PasswordInput
          autoComplete="off"
          value={apiKey}
          onChange={(event) => setApiKey(event.target.value)}
          required={spotRequired || apiSecret.trim() !== ""}
        />
      </label>
      <label className={shared.field}>
        <span>
          {kraken
            ? `Spot API secret (${spotRequired ? "required" : "optional"})`
            : binance
              ? "API secret or Ed25519 private key"
              : "API secret"}
        </span>
        {binance ? (
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
            autoComplete="off"
            value={apiSecret}
            onChange={(event) => setApiSecret(event.target.value)}
            required={spotRequired || apiKey.trim() !== ""}
          />
        )}
      </label>
      {binance ? (
        <p className={shared.muted}>
          Paste an unencrypted PKCS#8 Ed25519 private key PEM, including its BEGIN and END lines.
          Plutus detects it automatically; other values use HMAC. The private key is visible while
          you enter it and is encrypted when stored.
        </p>
      ) : null}
      {kraken ? (
        <>
          <p className={shared.muted}>
            Main requires both key groups to verify the wallet structure. Earn uses Spot
            credentials; independent derivatives accounts use Futures credentials.
          </p>
          <label className={shared.field}>
            <span>Futures API key ({futuresRequired ? "required" : "optional"})</span>
            <PasswordInput
              autoComplete="off"
              value={futuresApiKey}
              onChange={(event) => setFuturesApiKey(event.target.value)}
              required={futuresRequired || futuresApiSecret.trim() !== ""}
            />
          </label>
          <label className={shared.field}>
            <span>Futures API secret ({futuresRequired ? "required" : "optional"})</span>
            <PasswordInput
              autoComplete="off"
              value={futuresApiSecret}
              onChange={(event) => setFuturesApiSecret(event.target.value)}
              required={futuresRequired || futuresApiKey.trim() !== ""}
            />
          </label>
        </>
      ) : !binance ? (
        <label className={shared.field}>
          <span>{passphraseRequired ? "Passphrase" : "Passphrase (optional)"}</span>
          <PasswordInput
            autoComplete="off"
            value={passphrase}
            onChange={(event) => setPassphrase(event.target.value)}
            required={passphraseRequired}
          />
        </label>
      ) : null}
      <div className={shared.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          Store credentials
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
      {item.connection.credentialsConfigured ? (
        <div className={shared.actions}>
          {confirmRemove ? (
            <>
              <span className={shared.muted}>
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
