import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { useParams } from "react-router";
import {
  deleteCapitalFlow,
  getCapitalFlow,
  invalidatePortfolioReads,
  keys,
  listBoundaryEvents,
  listCapitalFlows,
  problemMessage,
  unwrapOrRedirect,
} from "@/api";
import type { BoundaryEventList, CapitalFlow, CapitalFlowList } from "@/api";
import { formatAmount, formatDecimal, parseDecimal } from "@/shared/lib/amount";
import { formatInstant, isoFromLocalInput } from "@/shared/lib/datetime";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import { useSessionScope } from "@/shared/lib/session";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { accountOptions, useIdentities } from "@/shared/lib/useIdentities";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DirectionBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import { NumericText } from "@/shared/ui/NumericText";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { CapitalFlowDialog, type FlowDialogRequest, flowEtag } from "../dialogs/CapitalFlowDialog";
import styles from "@/features/capital/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function CapitalPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const write = useAuthenticatedWrite();
  const queryClient = useQueryClient();
  const { params, update } = useSearchState();
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const eventsFrom = params.get("eventsFrom") ?? "";
  const eventsTo = params.get("eventsTo") ?? "";
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.timezone;
  const platforms = usePlatforms();
  const identities = useIdentities(portfolioId);
  const accounts = accountOptions(identities.data?.items);
  const nonce = useRef(0);
  const [dialog, setDialog] = useState<FlowDialogRequest | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<CapitalFlow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const flows = usePortfolioQuery<CapitalFlowList>({
    key: (userId, portfolioId) => keys.capitalFlows(userId, portfolioId, from, to),
    load: (portfolioId) =>
      listCapitalFlows({
        path: { portfolioId },
        query: { from: isoFromLocalInput(from), to: isoFromLocalInput(to) },
      }),
  });
  const events = usePortfolioQuery<BoundaryEventList>({
    key: (userId, portfolioId) => keys.boundaryEvents(userId, portfolioId, eventsFrom, eventsTo),
    load: (portfolioId) =>
      listBoundaryEvents({
        path: { portfolioId },
        query: { from: isoFromLocalInput(eventsFrom), to: isoFromLocalInput(eventsTo) },
      }),
  });

  async function invalidate() {
    await invalidatePortfolioReads(queryClient, userId, portfolioId);
  }

  function openCreate() {
    setError(null);
    setNotice(null);
    nonce.current += 1;
    setDialog({ nonce: nonce.current, mode: "create" });
  }

  async function openEdit(item: CapitalFlow) {
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const result = await getCapitalFlow({ path: { portfolioId, flowId: item.id } });
      const latest = unwrapOrRedirect(result, redirect);
      nonce.current += 1;
      setDialog({
        nonce: nonce.current,
        mode: "edit",
        item: latest,
      });
    } catch (cause) {
      setError(problemMessage(cause, "The Capital Flow could not be loaded."));
    } finally {
      setBusy(false);
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) {
      return;
    }
    const flowId = deleteTarget.id;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const result = await write(() =>
        deleteCapitalFlow({
          path: { portfolioId, flowId },
          headers: { "If-Match": flowEtag(deleteTarget) },
        }),
      );
      if (result.response?.status === 412) {
        setDeleteTarget(null);
        setError(
          "The flow changed in another session. Review the updated list before deleting it.",
        );
        await invalidate();
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The Capital Flow could not be deleted."));
        return;
      }
      setDeleteTarget(null);
      setNotice("Capital Flow deleted.");
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The Capital Flow could not be deleted."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={shared.wide}>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p className={shared.notice} role="status">
          {notice}
        </p>
      ) : null}
      <section className={shared.panel}>
        <div className={shared.panelHeader}>
          <div>
            <h2>Capital Flows</h2>
            <p className={shared.muted} style={{ margin: "2px 0 0" }}>
              Manual contributions and withdrawals recorded to adjust portfolio valuation.
            </p>
          </div>
          <ActionButton type="button" tone="primary" disabled={busy} onClick={openCreate}>
            Record flow
          </ActionButton>
        </div>
        <form className={shared.compactFilterBar} onSubmit={(event) => event.preventDefault()}>
          <label className={shared.compactFilterField}>
            <span className={shared.compactFilterLabel}>From</span>
            <TextInput
              type="datetime-local"
              value={from}
              className={styles.compactDateInput}
              onChange={(event) => update({ from: event.target.value })}
            />
          </label>
          <label className={shared.compactFilterField}>
            <span className={shared.compactFilterLabel}>To</span>
            <TextInput
              type="datetime-local"
              value={to}
              className={styles.compactDateInput}
              onChange={(event) => update({ to: event.target.value })}
            />
          </label>
          {from || to ? (
            <ActionButton type="button" tone="ghost" onClick={() => update({ from: "", to: "" })}>
              Clear
            </ActionButton>
          ) : null}
        </form>
        <QueryRecovery query={flows} fallback="Capital Flows could not be loaded." />
        <DataTable
          caption="Manual Capital Flows, newest first"
          columns={[
            { key: "effectiveAt", header: "Effective at", nowrap: true, width: "170px" },
            { key: "direction", header: "Direction", width: "110px" },
            { key: "canonicalAssetSymbol", header: "Canonical Asset", width: "140px" },
            { key: "quantity", header: "Quantity", numeric: true, width: "130px" },
            { key: "valueUsd", header: "Value (USD)", numeric: true, width: "140px" },
            { key: "account", header: "Account", width: "150px" },
            { key: "note", header: "Note" },
            { key: "actions", header: "", width: "80px" },
          ]}
          rows={(flows.data?.items ?? []).map((item) => {
            const dec = parseDecimal(item.valueUsd);
            const isContribution = item.direction.toLowerCase() === "contribution";
            return {
              id: item.id,
              effectiveAt: formatInstant(item.effectiveAt, timezone),
              direction: <DirectionBadge value={item.direction} />,
              canonicalAssetSymbol: item.canonicalAssetSymbol,
              quantity: formatDecimal(item.quantity),
              valueUsd: dec ? (
                <span className={isContribution ? shared.pnlPositive : shared.pnlNegative}>
                  <NumericText>{`${isContribution ? "+" : "-"}$${formatAmount(dec.abs().toFixed())}`}</NumericText>
                </span>
              ) : (
                <span className={shared.muted}>—</span>
              ),
              account:
                item.accountName ??
                (item.accountId ? item.accountId : <span className={shared.muted}>—</span>),
              note: item.note ?? "—",
              actions: (
                <span className={shared.rowActions}>
                  <ActionButton
                    type="button"
                    tone="ghost"
                    disabled={busy}
                    onClick={() => void openEdit(item)}
                  >
                    Edit
                  </ActionButton>
                  <ActionButton
                    type="button"
                    tone="dangerQuiet"
                    disabled={busy}
                    onClick={() => setDeleteTarget(item)}
                  >
                    Delete
                  </ActionButton>
                </span>
              ),
            };
          })}
          empty={
            flows.isPending
              ? "Loading Capital Flows."
              : "No Capital Flows recorded. Record a contribution or withdrawal to adjust Value Change."
          }
        />
      </section>
      <section className={shared.panel}>
        <div className={shared.panelHeader}>
          <div>
            <h2>Boundary Events</h2>
            <p className={shared.muted} style={{ margin: "2px 0 0" }}>
              Automatic adjustments recorded when accounts enter or leave the active boundary.
            </p>
          </div>
        </div>
        <form className={shared.compactFilterBar} onSubmit={(event) => event.preventDefault()}>
          <label className={shared.compactFilterField}>
            <span className={shared.compactFilterLabel}>From</span>
            <TextInput
              type="datetime-local"
              value={eventsFrom}
              className={styles.compactDateInput}
              onChange={(event) => update({ eventsFrom: event.target.value })}
            />
          </label>
          <label className={shared.compactFilterField}>
            <span className={shared.compactFilterLabel}>To</span>
            <TextInput
              type="datetime-local"
              value={eventsTo}
              className={styles.compactDateInput}
              onChange={(event) => update({ eventsTo: event.target.value })}
            />
          </label>
          {eventsFrom || eventsTo ? (
            <ActionButton
              type="button"
              tone="ghost"
              onClick={() => update({ eventsFrom: "", eventsTo: "" })}
            >
              Clear
            </ActionButton>
          ) : null}
        </form>
        <QueryRecovery query={events} fallback="Boundary Events could not be loaded." />
        <DataTable
          caption="Boundary Events in the window"
          columns={BOUNDARY_COLUMNS}
          rows={(events.data?.items ?? []).map((item) =>
            boundaryEventRow(item, timezone, platforms.data?.items),
          )}
          empty={
            events.isPending
              ? "Loading Boundary Events."
              : "No Boundary Events in this window. They appear when an account is added to or removed from the boundary between Valuation Points."
          }
        />
      </section>
      <CapitalFlowDialog
        portfolioId={portfolioId}
        request={dialog}
        accounts={accounts}
        onClose={() => setDialog(null)}
        onSaved={async (message) => {
          setDialog(null);
          setNotice(message);
          await invalidate();
        }}
      />
      <Dialog
        open={deleteTarget !== null}
        title="Delete Capital Flow"
        onClose={() => {
          if (!busy) setDeleteTarget(null);
        }}
      >
        <p className={shared.lead}>
          <NumericText>
            {deleteTarget
              ? `Delete the ${deleteTarget.direction} of ${formatDecimal(deleteTarget.quantity)} ${deleteTarget.canonicalAssetSymbol} effective ${formatInstant(deleteTarget.effectiveAt, timezone)}? Value Changes over that period will no longer account for it.`
              : ""}
          </NumericText>
        </p>
        <div className={shared.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={busy}
            onClick={() => void confirmDelete()}
          >
            Delete
          </ActionButton>
          <ActionButton
            type="button"
            tone="ghost"
            disabled={busy}
            onClick={() => setDeleteTarget(null)}
          >
            Cancel
          </ActionButton>
        </div>
      </Dialog>
    </section>
  );
}
