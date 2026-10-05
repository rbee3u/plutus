import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useId, useState } from "react";
import {
  createManualBalance,
  deleteManualBalance,
  getManualBalances,
  listAssetListings,
  updateManualBalance,
} from "@/api/generated/sdk.gen";
import type { ManualBalanceState } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { useIdentities } from "@/features/data/useIdentities";
import { parseDecimal } from "@/shared/lib/amount";
import { usePortfolio } from "@/shared/lib/portfolio";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { IconButton } from "@/shared/ui/IconButton";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

export interface ManualTarget {
  identityId: string;
  symbol?: string;
  action?: "edit" | "delete";
}

function useManual(portfolioId: string, identityId: string, fresh = false) {
  const { userId, redirect } = useSessionScope();
  const editorId = useId();
  const key = fresh
    ? ["manual-editor", userId, portfolioId, identityId, editorId]
    : ["manual-balances", userId, portfolioId, identityId];
  const query = useQuery({
    queryKey: key,
    enabled: Boolean(userId && portfolioId && identityId),
    refetchOnMount: fresh ? "always" : true,
    refetchOnWindowFocus: !fresh,
    queryFn: async () =>
      unwrapOrRedirect(await getManualBalances({ path: { portfolioId, identityId } }), redirect),
  });
  return { ...query, key };
}

export function ManualActions({
  identityId,
  symbol,
  onOpen,
}: {
  identityId: string;
  symbol: string;
  onOpen: (target: ManualTarget) => void;
}) {
  return (
    <div className={styles.actions}>
      <IconButton
        label={`Edit manual ${symbol}`}
        onClick={() => onOpen({ identityId, symbol, action: "edit" })}
      >
        <Pencil size={iconSize.sm} aria-hidden="true" />
      </IconButton>
      <IconButton
        label={`Delete manual ${symbol}`}
        tone="dangerQuiet"
        onClick={() => onOpen({ identityId, symbol, action: "delete" })}
      >
        <Trash2 size={iconSize.sm} aria-hidden="true" />
      </IconButton>
    </div>
  );
}

/** Mutable changes are displayed separately from immutable valuation rows. */
export function manualChanges(state: ManualBalanceState) {
  const before = new Map(state.appliedItems.map((item) => [item.symbol, item.quantity]));
  const after = new Map(state.items.map((item) => [item.symbol, item.quantity]));
  return [...new Set([...before.keys(), ...after.keys()])].sort().flatMap((symbol) => {
    const oldQuantity = before.get(symbol) ?? "0";
    const quantity = after.get(symbol) ?? "0";
    return parseDecimal(oldQuantity)?.equals(parseDecimal(quantity) ?? "0")
      ? []
      : [{ symbol, oldQuantity, quantity }];
  });
}

export function ManualPending({
  portfolioId,
  identityId,
  name,
  onOpen,
}: {
  portfolioId: string;
  identityId: string;
  name?: string;
  onOpen: (target: ManualTarget) => void;
}) {
  const query = useManual(portfolioId, identityId);
  return (
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Manual records could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      {query.data?.pending ? (
        <div className={styles.notice}>
          <div className={styles.headingRow}>
            <strong>{name ? `${name} · ` : ""}Manual changes awaiting collection</strong>
            <ActionButton tone="ghost" onClick={() => onOpen({ identityId })}>
              Manage records
            </ActionButton>
          </div>
          <DataTable
            caption="Pending manual quantities"
            columns={[
              { key: "symbol", header: "Asset" },
              { key: "oldQuantity", header: "Last collected", numeric: true },
              { key: "quantity", header: "Next collection", numeric: true },
            ]}
            rows={manualChanges(query.data).map((item) => ({ ...item, id: item.symbol }))}
          />
          <p>These changes are excluded from the current Valuation Point.</p>
        </div>
      ) : null}
    </>
  );
}

export function ManualDialog({
  portfolioId,
  target,
  onClose,
}: {
  portfolioId: string;
  target: ManualTarget | null;
  onClose: () => void;
}) {
  return (
    <Dialog open={target !== null} title="Manual balances" onClose={onClose}>
      {target ? (
        <ManualEditor
          key={`${target.identityId}:${target.symbol ?? ""}:${target.action ?? "add"}`}
          portfolioId={portfolioId}
          target={target}
        />
      ) : null}
    </Dialog>
  );
}

function ManualEditor({ portfolioId, target }: { portfolioId: string; target: ManualTarget }) {
  const identities = useIdentities(portfolioId);
  const portfolio = usePortfolio(portfolioId);
  const [identityId, setIdentityId] = useState(target.identityId);
  const query = useManual(portfolioId, identityId, true);
  const identity = identities.data?.items.find((item) => item.id === identityId);
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const symbolsId = useId();
  const listings = useQuery({
    queryKey: ["manual-symbols", userId, portfolioId, identity?.platform],
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
    <>
      <p>
        Record only assets or liabilities missing from the Platform API. Update or remove them when
        the API includes them. Saved quantities apply at the next Collection Cycle.
      </p>
      <label className={styles.field}>
        Identity
        <Select
          value={identityId}
          onChange={(event) => setIdentityId(event.target.value)}
          disabled={Boolean(target.identityId)}
        >
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
      <QueryRecovery
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
            await refreshCsrf();
            const path = { portfolioId, identityId, symbol };
            const headers = { "If-Match": `"manual:${query.data.version}"` };
            const result =
              action === "delete"
                ? await deleteManualBalance({ path, headers })
                : action === "edit"
                  ? await updateManualBalance({ path, headers, body: { quantity } })
                  : await createManualBalance({
                      path: { portfolioId, identityId },
                      headers,
                      body: { symbol, quantity },
                    });
            if (result.response?.status === 412)
              throw new Error(
                "Manual records changed elsewhere. Reload current records before saving again.",
              );
            const state = unwrapOrRedirect(result, redirect);
            queryClient.setQueryData(query.key, state);
            await queryClient.invalidateQueries({
              queryKey: ["manual-balances", userId, portfolioId, identityId],
            });
          }}
        />
      ) : null}
      <datalist id={symbolsId}>
        {[...new Set((listings.data?.items ?? []).map((item) => item.symbol))].map((symbol) => (
          <option key={symbol} value={symbol} />
        ))}
      </datalist>
    </>
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
    } catch (error) {
      setMessage(problemMessage(error, "The manual record could not be saved."));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <form
        className={styles.form}
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        <label className={styles.field}>
          Asset code
          <TextInput
            list={symbolsId}
            value={symbol}
            disabled={action !== "add" || busy}
            required
            maxLength={64}
            onChange={(event) => setSymbol(event.target.value)}
          />
        </label>
        {action === "delete" ? (
          <p>Remove {symbol} from future collections? Earlier reports remain unchanged.</p>
        ) : (
          <label className={styles.field}>
            Quantity
            <TextInput
              value={quantity}
              required
              inputMode="text"
              disabled={busy}
              onChange={(event) => setQuantity(event.target.value)}
            />
            <small>Negative quantities record liabilities. Zero removes the record.</small>
          </label>
        )}
        {action === "add" && existing ? (
          <p role="status">
            This asset already exists.{" "}
            <ActionButton
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
        {message ? <p role={saved ? "status" : "alert"}>{message}</p> : null}
        <div className={styles.actions}>
          <ActionButton
            type="submit"
            tone={action === "delete" ? "danger" : "primary"}
            disabled={busy || (action === "add" && Boolean(existing))}
          >
            {action === "delete" ? "Delete record" : "Save quantity"}
          </ActionButton>
          <ActionButton tone="ghost" disabled={busy} onClick={onReload}>
            Reload current records
          </ActionButton>
        </div>
      </form>
      <h3>Current editable records</h3>
      <p className={styles.muted}>
        Includes pending and excluded records. Prices and values belong to collected reports. Times
        in {timezone}.
      </p>
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
          symbol: item.symbol,
          quantity: item.quantity,
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
              }}
            />
          ),
        }))}
        empty="No manual records. Enter an asset and quantity above."
      />
      <ActionButton
        tone="ghost"
        disabled={busy}
        onClick={() => {
          setSymbol("");
          setQuantity("");
          setAction("add");
          setMessage("");
        }}
      >
        <Plus size={iconSize.sm} aria-hidden="true" /> Add another asset
      </ActionButton>
    </>
  );
}
