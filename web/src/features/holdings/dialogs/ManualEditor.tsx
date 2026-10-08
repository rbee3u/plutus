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
import { RequestRecovery } from "@/shared/ui/QueryRecovery";
import shared from "@/shared/styles/page.module.css";

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
    <>
      <p>
        Record only assets or liabilities missing from the Platform API. Update or remove them when
        the API includes them. Saved quantities apply at the next Collection Cycle.
      </p>
      <label className={shared.field}>
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
        className={shared.form}
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        <label className={shared.field}>
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
          <label className={shared.field}>
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
        <div className={shared.actions}>
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
      <p className={shared.muted}>
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
