import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router";
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
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import shared from "@/shared/styles/page.module.css";
import styles from "./ManualEditor.module.css";

class ManualConflictError extends Error {}

export function ManualEditor({
  portfolioId,
  target,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  target: ManualTarget;
  onClose?: () => void;
  onSaved?: () => void;
}) {
  const identities = useIdentities(portfolioId);
  const [identityId, setIdentityId] = useState(target.identityId);
  const query = useManual(portfolioId, identityId);
  const identity = identities.data?.items.find((item) => item.id === identityId);
  const availableIdentities = identities.data?.items.filter((item) => item.status !== "archived");
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
      ) : identities.isSuccess && availableIdentities?.length === 0 ? (
        <TableEmptyState
          title={
            identities.data.items.length === 0 ? "No identities yet" : "No identities available"
          }
          help="Add an identity before adding a manual balance."
          action={
            <Link to={`/p/${portfolioId}/data/identities`} onClick={onClose}>
              Manage identities
            </Link>
          }
        />
      ) : (
        <label className={styles.field}>
          <span>Identity</span>
          <Select value={identityId} onChange={(event) => setIdentityId(event.target.value)}>
            <option value="">Choose an identity</option>
            {availableIdentities?.map((item) => (
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
      {identityId && !query.data && query.isFetching ? (
        <p role="status">Loading current manual records…</p>
      ) : query.data && identityId ? (
        <ManualForm
          key={identityId}
          state={query.data}
          target={target}
          onClose={onClose}
          onReload={async () => {
            const result = await query.refetch();
            if (result.error) throw result.error;
            return result.data!;
          }}
          onSave={async (symbol, quantity, action, version) => {
            const path = { portfolioId, identityId, assetSymbol: symbol };
            const headers = { "If-Edit-Version": String(version) };
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
                    body: { assetSymbol: symbol, quantity },
                  });
              }
            });
            if (result.response?.status === 412) {
              throw new ManualConflictError(
                "Manual records changed elsewhere. Reload current records before saving again.",
              );
            }
            const state = unwrapOrRedirect(result, redirect);
            queryClient.setQueryData(query.key, state);
            await Promise.all([
              queryClient.invalidateQueries({ queryKey: keys.balances(userId, portfolioId) }),
              queryClient.invalidateQueries({
                queryKey: keys.accountReport(userId, portfolioId),
              }),
              queryClient.invalidateQueries({ queryKey: keys.overview(userId, portfolioId) }),
            ]);
            onSaved?.();
            return state;
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
  onSave: (
    symbol: string,
    quantity: string,
    action: "add" | "edit" | "delete",
    version: number,
  ) => Promise<ManualBalanceState>;
  onReload: () => Promise<ManualBalanceState>;
  onClose?: () => void;
  onSaved?: () => void;
}) {
  const [baseline, setBaseline] = useState(state);
  const initial = baseline.items.find((item) => item.assetSymbol === target.symbol);
  const [symbol, setSymbol] = useState(initial?.assetSymbol ?? "");
  const [quantity, setQuantity] = useState(initial?.quantity ?? "");
  const action = target.action ?? (initial ? "edit" : "add");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(
    target.symbol && !initial && action !== "add" ? "This record has already been removed." : "",
  );
  const [saved, setSaved] = useState(false);
  const [needsReload, setNeedsReload] = useState(false);
  const existing =
    !saved &&
    Boolean(baseline.items.find((item) => item.assetSymbol === symbol.trim().toUpperCase()));

  async function save() {
    if (busy || needsReload) return;
    setBusy(true);
    setMessage("");
    setSaved(false);
    try {
      const latest = await onSave(symbol.trim().toUpperCase(), quantity, action, baseline.version);
      setBaseline(latest);
      setSaved(true);
      setMessage("Saved. The next collection cycle will apply this quantity.");
      onClose?.();
    } catch (error) {
      setMessage(problemMessage(error, "The manual record could not be saved."));
      setNeedsReload(error instanceof ManualConflictError);
    } finally {
      setBusy(false);
    }
  }

  async function reload() {
    setBusy(true);
    try {
      const latest = await onReload();
      setBaseline(latest);
      if (action !== "add") {
        setQuantity(latest.items.find((item) => item.assetSymbol === symbol)?.quantity ?? "");
      }
      setMessage("");
      setNeedsReload(false);
      setSaved(false);
    } catch (error) {
      setMessage(problemMessage(error, "Manual records could not be loaded."));
    } finally {
      setBusy(false);
    }
  }

  if (action !== "add" && !initial && !saved && !busy) {
    return <p role="status">This record has already been removed.</p>;
  }

  const feedback = message ? (
    <p
      className={saved ? styles.statusSuccess : styles.statusError}
      role={saved ? "status" : "alert"}
    >
      {message}
    </p>
  ) : null;
  const actions = (
    <div className={shared.dialogActions}>
      {!saved ? (
        <span className={shared.dialogActionLead}>
          <ActionButton type="button" tone="ghost" disabled={busy} onClick={() => void reload()}>
            Reload current records
          </ActionButton>
        </span>
      ) : null}
      {!saved && onClose ? (
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      ) : null}
      {saved ? (
        onClose ? (
          <ActionButton type="button" tone="primary" onClick={onClose}>
            Done
          </ActionButton>
        ) : null
      ) : (
        <ActionButton
          type="submit"
          tone={action === "delete" ? "danger" : "primary"}
          disabled={busy || needsReload || (action === "add" && existing)}
        >
          {action === "delete" ? "Delete record" : "Save quantity"}
        </ActionButton>
      )}
    </div>
  );

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
            {feedback}
            {actions}
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
                  if (saved) {
                    setSaved(false);
                    setMessage("");
                  }
                }}
              />
            </label>
            {feedback}
            {actions}
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
                    if (saved) {
                      setSaved(false);
                      setMessage("");
                    }
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
                    if (saved) {
                      setSaved(false);
                      setMessage("");
                    }
                  }}
                />
              </label>
            </div>
            {existing ? (
              <p role="status" className={styles.fieldNote}>
                This asset already exists in this identity.
              </p>
            ) : null}
            {feedback}
            {actions}
          </form>
        </div>
      )}
    </>
  );
}
