import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useState } from "react";
import { patchIdentity, problemMessage } from "@/api";
import type { Identity } from "@/api";
import { CONFLICT } from "./identityCommon";
import { identityEtag } from "@/shared/lib/useIdentities";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput, Toggle } from "@/shared/ui/FormControls";
import styles from "@/features/manage/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function EditIdentityForm({
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
  const write = useAuthenticatedWrite();
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
      const result = await write(() =>
        patchIdentity({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
          body: {
            displayName: displayName.trim(),
            accounts: item.accounts.map((account) => ({
              id: account.id,
              selected: selected[account.id] ?? account.selected,
            })),
          },
        }),
      );
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
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)}>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={shared.field}>
        <span>Display name</span>
        <TextInput
          value={displayName}
          onChange={(event) => setDisplayName(event.target.value)}
          maxLength={120}
          required
        />
      </label>
      <fieldset className={shared.field}>
        <legend>
          <span>Selected accounts</span>
        </legend>
        <div className={styles.checkList}>
          {item.accounts.map((account) => (
            <Toggle
              key={account.id}
              disabled={account.accountType === "manual"}
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
      <div className={shared.actions}>
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
