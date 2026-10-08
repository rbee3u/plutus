import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useId, useState } from "react";
import {
  createManualBalance,
  deleteManualBalance,
  keys,
  listAssetListings,
  problemMessage,
  unwrapOrRedirect,
  updateManualBalance,
} from "@/api";
import type { ManualBalanceState } from "@/api";
import { ManualActions, useManual, type ManualTarget } from "./manual";
import { useIdentities } from "@/shared/lib/useIdentities";
import { usePortfolio } from "@/shared/lib/portfolio";
import { formatInstant } from "@/shared/lib/datetime";
import { useSessionScope } from "@/shared/lib/session";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { NumericText } from "@/shared/ui/NumericText";
import { RequestRecovery } from "@/shared/ui/QueryRecovery";
import styles from "./ManualEditor.module.css";

export function ManualEditor({
  portfolioId,
  target,
}: {
  portfolioId: string;
  target: ManualTarget;
}) {
  const identities = useIdentities(portfolioId);
  const portfolio = usePortfolio(portfolioId);
  const [identityId, setIdentityId] = useState(target.identityId);
  const query = useManual(portfolioId, identityId, true);
  const identity = identities.data?.items.find((item) => item.id === identityId);
  const { userId, redirect } = useSessionScope();
  const write = useAuthenticatedWrite();
  const queryClient = useQueryClient();
  const symbolsId = useId();
  const listings = useQuery({
    queryKey: keys.manualSymbols(userId, portfolioId, identity?.platform),
    enabled: Boolean(userId && identity),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAssetListings({
          path: { portfolioId },
          query: { platform: identity?.platform, limit: 200 },
        }),
        redirect,
      ),
  });
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
          symbolsId={symbolsId}
          timezone={portfolio.data?.item.timezone ?? "UTC"}
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
      <datalist id={symbolsId}>
        {[...new Set((listings.data?.items ?? []).map((item) => item.symbol))].map((symbol) => (
          <option key={symbol} value={symbol} />
        ))}
      </datalist>
    </div>
  );
}

function ManualForm({
  state,
  target,
  symbolsId,
  timezone,
  onSave,
  onReload,
}: {
  state: ManualBalanceState;
  target: ManualTarget;
  symbolsId: string;
  timezone: string;
  onSave: (symbol: string, quantity: string, action: "add" | "edit" | "delete") => Promise<void>;
  onReload: () => void;
}) {
  const initial = state.items.find((item) => item.symbol === target.symbol);
  const [symbol, setSymbol] = useState(initial?.symbol ?? "");
  const [quantity, setQuantity] = useState(initial?.quantity ?? "");
  const [action, setAction] = useState<"add" | "edit" | "delete">(
    initial ? (target.action ?? "edit") : "add",
  );
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(
    target.symbol && !initial
      ? "This record has already been removed. Current records are shown below."
      : "",
  );
  const [saved, setSaved] = useState(false);
  const existing = state.items.find((item) => item.symbol === symbol.trim().toUpperCase());

  async function save() {
    setBusy(true);
    setMessage("");
    setSaved(false);
    try {
      await onSave(symbol.trim().toUpperCase(), quantity, action);
      setSaved(true);
      setMessage("Saved. The next Collection Cycle will apply this quantity.");
      if (action === "delete") {
        setAction("add");
        setSymbol("");
        setQuantity("");
      }
    } catch (error) {
      setMessage(problemMessage(error, "The manual record could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  function switchToAdd() {
    setSymbol("");
    setQuantity("");
    setAction("add");
    setMessage("");
    setSaved(false);
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
              <p className={styles.statusError} role="alert">
                {message}
              </p>
            ) : null}
            <div className={styles.cardActions}>
              <ActionButton type="submit" tone="danger" disabled={busy}>
                Delete record
              </ActionButton>
              <ActionButton type="button" tone="ghost" disabled={busy} onClick={switchToAdd}>
                Cancel
              </ActionButton>
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
            <ActionButton type="button" tone="ghost" disabled={busy} onClick={switchToAdd}>
              Cancel
            </ActionButton>
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
                placeholder="0.00 (negative for liability)"
                required
                inputMode="text"
                disabled={busy}
                onChange={(event) => setQuantity(event.target.value)}
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
              <ActionButton type="submit" tone="primary" disabled={busy}>
                Save quantity
              </ActionButton>
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
                  list={symbolsId}
                  value={symbol}
                  placeholder="e.g. USD, BTC"
                  disabled={busy}
                  required
                  maxLength={64}
                  onChange={(event) => setSymbol(event.target.value)}
                />
              </label>
              <label className={styles.field}>
                <span>Quantity</span>
                <TextInput
                  value={quantity}
                  placeholder="0.00 (negative for liability)"
                  required
                  inputMode="text"
                  disabled={busy}
                  onChange={(event) => setQuantity(event.target.value)}
                />
              </label>
            </div>
            {existing ? (
              <p role="status" className={styles.fieldNote}>
                This asset already exists.{" "}
                <ActionButton
                  type="button"
                  tone="ghost"
                  onClick={() => {
                    setAction("edit");
                    setSymbol(existing.symbol);
                    setQuantity(existing.quantity);
                  }}
                >
                  Edit current quantity
                </ActionButton>
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
              <ActionButton type="submit" tone="primary" disabled={busy || Boolean(existing)}>
                Save quantity
              </ActionButton>
            </div>
          </form>
        </div>
      )}

      <div className={styles.recordsSection}>
        <div className={styles.recordsHeader}>
          <div className={styles.recordsTitleGroup}>
            <h3 className={styles.recordsTitle}>Current records</h3>
            <span className={styles.recordsCount}>{state.items.length}</span>
          </div>
          {action !== "add" ? (
            <ActionButton type="button" tone="ghost" disabled={busy} onClick={switchToAdd}>
              <Plus size={iconSize.sm} aria-hidden="true" /> Add another asset
            </ActionButton>
          ) : null}
        </div>
        <DataTable
          caption="Current manual records"
          columns={[
            { key: "symbol", header: "Asset" },
            { key: "quantity", header: "Quantity", numeric: true },
            { key: "modified", header: "Last edited" },
            { key: "actions", header: "Actions" },
          ]}
          rows={state.items.map((item) => ({
            id: item.symbol,
            symbol: <span className={styles.assetSymbol}>{item.symbol}</span>,
            quantity: <NumericText>{item.quantity}</NumericText>,
            modified: formatInstant(item.modifiedAt, timezone),
            actions: (
              <ManualActions
                identityId={state.identityId}
                symbol={item.symbol}
                onOpen={(next) => {
                  setSymbol(item.symbol);
                  setQuantity(item.quantity);
                  setAction(next.action ?? "edit");
                  setMessage("");
                  setSaved(false);
                }}
              />
            ),
          }))}
          empty="No manual records. Enter an asset and quantity above."
        />
        <p className={styles.footerNote}>
          Saved quantities take effect at the next collection cycle.
        </p>
      </div>
    </>
  );
}
