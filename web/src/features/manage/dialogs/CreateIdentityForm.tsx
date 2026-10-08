import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useRef, useState } from "react";
import { createIdentity, problemMessage } from "@/api";
import type { AccountStructure, Platform } from "@/api";
import { structureLabel } from "@/shared/lib/usePlatforms";
import { newIdempotencyKey } from "@/shared/lib/id";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Select, TextInput, Toggle } from "@/shared/ui/FormControls";
import styles from "@/features/manage/styles.module.css";
import shared from "@/shared/styles/page.module.css";

/** The Account Structure to preselect: a Platform with a single structure needs no choice. */
function onlyStructure(platform: Platform | undefined): AccountStructure | undefined {
  return platform?.accountStructures.length === 1 ? platform.accountStructures[0] : undefined;
}

export function CreateIdentityForm({
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
  const write = useAuthenticatedWrite();
  const createKey = useRef(newIdempotencyKey());
  const firstPlatform = platforms[0];
  const [platformId, setPlatformId] = useState(firstPlatform?.id ?? "");
  const [structureId, setStructureId] = useState(onlyStructure(firstPlatform)?.id ?? "");
  const [selectedTypes, setSelectedTypes] = useState<string[]>(
    () => onlyStructure(firstPlatform)?.accountTypes.map((type) => type.id) ?? [],
  );
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const platform = platforms.find((item) => item.id === platformId);
  const structure = platform?.accountStructures.find((item) => item.id === structureId);

  function choosePlatform(nextId: string) {
    const only = onlyStructure(platforms.find((item) => item.id === nextId));
    setPlatformId(nextId);
    setStructureId(only?.id ?? "");
    setSelectedTypes(only?.accountTypes.map((type) => type.id) ?? []);
  }

  function chooseStructure(nextId: string) {
    const next = platform?.accountStructures.find((item) => item.id === nextId);
    setStructureId(nextId);
    setSelectedTypes(next?.accountTypes.map((type) => type.id) ?? []);
  }

  function toggleType(typeId: string, checked: boolean) {
    setSelectedTypes((current) =>
      checked ? [...new Set([...current, typeId])] : current.filter((id) => id !== typeId),
    );
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!structure) {
      setError("Choose the account structure; it cannot be changed after creation.");
      return;
    }
    if (displayName.trim() === "") {
      setError("Display name is required.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        createIdentity({
          path: { portfolioId },
          headers: { "Idempotency-Key": createKey.current },
          body: {
            platform: platformId,
            displayName: displayName.trim(),
            accountStructure: structureId,
            selectedAccountTypes: selectedTypes,
          },
        }),
      );
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The identity could not be created."));
        return;
      }
      createKey.current = newIdempotencyKey();
      await onSaved(
        `Identity ${result.data.displayName} created. Manual balances are available; add credentials to collect Platform accounts.`,
      );
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be created."));
    } finally {
      setBusy(false);
    }
  }

  if (platforms.length === 0) {
    return <p className={shared.notice}>No Platforms are available on this server.</p>;
  }

  return (
    <form className={shared.form} onSubmit={(event) => void onSubmit(event)}>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      <label className={shared.field}>
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
        <p className={shared.notice}>
          This server has no adapter for {platform.name}; the identity can be created but not
          collected.
        </p>
      ) : null}
      <label className={shared.field}>
        <span>Account structure</span>
        <Select
          value={structureId}
          onChange={(event) => chooseStructure(event.target.value)}
          required
        >
          {(platform?.accountStructures.length ?? 0) > 1 ? (
            <option value="" disabled>
              Choose the structure of this login
            </option>
          ) : null}
          {(platform?.accountStructures ?? []).map((item) => (
            <option key={item.id} value={item.id}>
              {structureLabel(platforms, platformId, item.id)}
            </option>
          ))}
        </Select>
      </label>
      <fieldset className={shared.field}>
        <legend>
          <span>Account types</span>
        </legend>
        <p className={shared.muted}>
          Every Identity also includes an always-selected Manual account.
        </p>
        {structure ? null : <p className={shared.muted}>Choose the account structure first.</p>}
        <div className={styles.checkList}>
          {(structure?.accountTypes ?? []).map((type) => (
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
      <label className={shared.field}>
        <span>Display name</span>
        <TextInput
          value={displayName}
          onChange={(event) => setDisplayName(event.target.value)}
          maxLength={120}
          required
        />
      </label>
      <div className={shared.actions}>
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
