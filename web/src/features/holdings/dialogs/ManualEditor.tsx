import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  createManualBalance,
  deleteManualBalance,
  keys,
  problemMessage,
  unwrapOrRedirect,
  updateManualBalance,
} from "@/api";
import type { ManualBalanceState } from "@/api";
import { useManual, type ManualTarget } from "./manual";
import { useIdentities } from "@/shared/lib/useIdentities";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { RequestRecovery } from "@/shared/ui/QueryRecovery";
import styles from "./ManualEditor.module.css";

export function ManualEditor({
  portfolioId,
  target,
  onClose,
}: {
  portfolioId: string;
  target: ManualTarget;
  onClose?: () => void;
}) {
  const identities = useIdentities(portfolioId);
  const [identityId, setIdentityId] = useState(target.identityId);
  const query = useManual(portfolioId, identityId, true);
  const identity = identities.data?.items.find((item) => item.id === identityId);
  const { userId, redirect } = useSessionScope();
  const write = useAuthenticatedWrite();
  const queryClient = useQueryClient();

  return (
    <div className={styles.container}>
      {target.identityId ? (
        <div className={styles.identityBar}>
          <span className={styles.identityLabel}>Identity</span>
          <span className={styles.identityBadge}>{identity?.displayName ?? target.identityId}</span>
        </div>
      ) : (
        <label className={styles.field}>
          <span>Identity</span>
          <Select value={identityId} onChange={(event) => setIdentityId(event.target.value)}>
            <option value="">Choose an Identity</option>
            {identities.data?.items
              .filter((item) => item.status !== "archived")
              .map((item) => (
                <option key={item.id} value={item.id}>
                  {item.displayName}
                </option>
              ))}
          </Select>
        </label>
      )}
      <RequestRecovery
        error={query.error ?? identities.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Manual records could not be loaded."
        onRetry={() => {
          void query.refetch();
          void identities.refetch();
        }}
      />
      {identityId && query.isFetching ? (
        <p role="status">Loading current manual records…</p>
      ) : query.data && identityId ? (
        <ManualForm
          key={identityId}
          state={query.data}
          target={target}
          onClose={onClose}
          onReload={() => {
            void query.refetch();
          }}
          onSave={async (symbol, quantity, action) => {
            const path = { portfolioId, identityId, symbol };
            const headers = { "If-Match": `"manual:${query.data.version}"` };
            const result = await write(() => {
              switch (action) {
                case "delete":
                  return deleteManualBalance({ path, headers });
                case "edit":
                  return updateManualBalance({ path, headers, body: { quantity } });
                case "add":
                  return createManualBalance({
                    path: { portfolioId, identityId },
                    headers,
                    body: { symbol, quantity },
                  });
              }
            });
            if (result.response?.status === 412) {
              throw new Error(
                "Manual records changed elsewhere. Reload current records before saving again.",
              );
            }
            const state = unwrapOrRedirect(result, redirect);
            queryClient.setQueryData(query.key, state);
            await queryClient.invalidateQueries({
              queryKey: keys.manualBalances(userId, portfolioId, identityId),
            });
          }}
        />
      ) : null}
    </div>
  );
}

function ManualForm({
  state,
  target,
  onSave,
  onReload,
  onClose,
}: {
  state: ManualBalanceState;
  target: ManualTarget;
  onSave: (symbol: string, quantity: string, action: "add" | "edit" | "delete") => Promise<void>;
  onReload: () => void;
  onClose?: () => void;
}) {
  const initial = state.items.find((item) => item.symbol === target.symbol);
  const [symbol, setSymbol] = useState(initial?.symbol ?? "");
  const [quantity, setQuantity] = useState(initial?.quantity ?? "");
  const action = target.action ?? (initial ? "edit" : "add");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(
    target.symbol && !initial && action !== "add" ? "This record has already been removed." : "",
  );
  const [saved, setSaved] = useState(false);
  const existing =
    !saved && Boolean(state.items.find((item) => item.symbol === symbol.trim().toUpperCase()));

  async function save() {
    setBusy(true);
    setMessage("");
    setSaved(false);
    try {
      await onSave(symbol.trim().toUpperCase(), quantity, action);
      setSaved(true);
      setMessage("Saved. The next Collection Cycle will apply this quantity.");
      if (action === "delete") {
        setSymbol("");
        setQuantity("");
      }
    } catch (error) {
      setMessage(problemMessage(error, "The manual record could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {action === "delete" ? (
        <div className={`${styles.card} ${styles.cardDanger}`}>
          <div className={styles.cardHeader}>
            <h3 className={styles.cardTitle}>
              Remove <strong>{symbol}</strong>
            </h3>
          </div>
          <form
            className={styles.form}
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <p className={styles.deletePrompt}>
              Remove {symbol} from future collections? Earlier reports remain unchanged.
            </p>
            {message ? (
              <p
                className={saved ? styles.statusSuccess : styles.statusError}
                role={saved ? "status" : "alert"}
              >
                {message}
              </p>
            ) : null}
            <div className={styles.cardActions}>
              {saved && onClose ? (
                <ActionButton type="button" tone="primary" onClick={onClose}>
                  Done
                </ActionButton>
              ) : (
                <ActionButton type="submit" tone="danger" disabled={busy || saved}>
                  Delete record
                </ActionButton>
              )}
              {onClose && !saved ? (
                <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
                  Cancel
                </ActionButton>
              ) : null}
              <ActionButton type="button" tone="ghost" disabled={busy} onClick={onReload}>
                Reload current records
              </ActionButton>
            </div>
          </form>
        </div>
      ) : action === "edit" ? (
        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <h3 className={styles.cardTitle}>
              Edit <strong>{symbol}</strong> quantity
            </h3>
          </div>
          <form
            className={styles.form}
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <label className={styles.field}>
              <span>Quantity</span>
              <TextInput
                value={quantity}
                placeholder="0.00 (negative for liability, 0 to clear)"
                required
                inputMode="text"
                disabled={busy}
                onChange={(event) => {
                  setQuantity(event.target.value);
                  if (saved) setSaved(false);
                }}
              />
            </label>
            {message ? (
              <p
                className={saved ? styles.statusSuccess : styles.statusError}
                role={saved ? "status" : "alert"}
              >
                {message}
              </p>
            ) : null}
            <div className={styles.cardActions}>
              {saved && onClose ? (
                <ActionButton type="button" tone="primary" onClick={onClose}>
                  Done
                </ActionButton>
              ) : (
                <ActionButton type="submit" tone="primary" disabled={busy}>
                  Save quantity
                </ActionButton>
              )}
              {onClose && !saved ? (
                <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
                  Cancel
                </ActionButton>
              ) : null}
              <ActionButton type="button" tone="ghost" disabled={busy} onClick={onReload}>
                Reload current records
              </ActionButton>
            </div>
          </form>
        </div>
      ) : (
        <div className={styles.card}>
          <div className={styles.cardHeader}>
            <h3 className={styles.cardTitle}>Add asset</h3>
          </div>
          <form
            className={styles.form}
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <div className={styles.formGrid}>
              <label className={styles.field}>
                <span>Asset code</span>
                <TextInput
                  value={symbol}
                  placeholder="e.g. USD, BTC"
                  disabled={busy || saved}
                  required
                  maxLength={64}
                  onChange={(event) => {
                    setSymbol(event.target.value);
                    if (saved) setSaved(false);
                  }}
                />
              </label>
              <label className={styles.field}>
                <span>Quantity</span>
                <TextInput
                  value={quantity}
                  placeholder="0.00 (negative for liability)"
                  required
                  inputMode="text"
                  disabled={busy || saved}
                  onChange={(event) => {
                    setQuantity(event.target.value);
                    if (saved) setSaved(false);
                  }}
                />
              </label>
            </div>
            {existing ? (
              <p role="status" className={styles.fieldNote}>
                This asset already exists in this identity.
              </p>
            ) : null}
            {message ? (
              <p
                className={saved ? styles.statusSuccess : styles.statusError}
                role={saved ? "status" : "alert"}
              >
                {message}
              </p>
            ) : null}
            <div className={styles.cardActions}>
              {saved ? (
                <>
                  {onClose ? (
                    <ActionButton type="button" tone="primary" onClick={onClose}>
                      Done
                    </ActionButton>
                  ) : null}
                  <ActionButton
                    type="button"
                    tone="ghost"
                    onClick={() => {
                      setSymbol("");
                      setQuantity("");
                      setSaved(false);
                      setMessage("");
                    }}
                  >
                    Add another asset
                  </ActionButton>
                </>
              ) : (
                <>
                  <ActionButton type="submit" tone="primary" disabled={busy || Boolean(existing)}>
                    Save quantity
                  </ActionButton>
                  {onClose ? (
                    <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
                      Cancel
                    </ActionButton>
                  ) : null}
                </>
              )}
            </div>
          </form>
        </div>
      )}
    </>
  );
}
