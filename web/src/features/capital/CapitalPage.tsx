import { NumericText } from "@/shared/ui/NumericText";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { useParams } from "react-router";
import {
  deleteCapitalFlow,
  getCapitalFlow,
  listBoundaryEvents,
  listCapitalFlows,
} from "@/api/generated/sdk.gen";
import type { CapitalFlow } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import {
  CapitalFlowDialog,
  type FlowDialogRequest,
  flowEtag,
} from "@/features/capital/CapitalFlowDialog";
import { accountOptions, useIdentities } from "@/features/data/useIdentities";
import { usePlatforms } from "@/features/data/usePlatforms";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/features/history/ValuationPointsPage";
import { formatAmount, formatDecimal, parseDecimal } from "@/shared/lib/amount";
import { formatInstant, isoFromLocalInput } from "@/shared/lib/datetime";
import { usePortfolio } from "@/shared/lib/portfolio";
import { useSearchState } from "@/shared/lib/searchParams";
import { readEtag, unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DirectionBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const AFFECTED_KEYS = [
  "capital-flows",
  "value-changes",
  "custom-value-change",
  "overview",
] as const;

export function CapitalPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const { params, update } = useSearchState();
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";
  const eventsFrom = params.get("eventsFrom") ?? "";
  const eventsTo = params.get("eventsTo") ?? "";
  const portfolio = usePortfolio(portfolioId);
  const timezone = portfolio.data?.item.timezone;
  const platforms = usePlatforms();
  const identities = useIdentities(portfolioId);
  const accounts = accountOptions(identities.data?.items);
  const nonce = useRef(0);
  const [dialog, setDialog] = useState<FlowDialogRequest | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<CapitalFlow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const flows = useQuery({
    queryKey: ["capital-flows", userId, portfolioId, from, to],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listCapitalFlows({
          path: { portfolioId },
          query: { from: isoFromLocalInput(from), to: isoFromLocalInput(to) },
        }),
        redirect,
      ),
  });
  const events = useQuery({
    queryKey: ["boundary-events", userId, portfolioId, eventsFrom, eventsTo],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listBoundaryEvents({
          path: { portfolioId },
          query: { from: isoFromLocalInput(eventsFrom), to: isoFromLocalInput(eventsTo) },
        }),
        redirect,
      ),
  });

  async function invalidate() {
    await Promise.all(
      AFFECTED_KEYS.map((key) =>
        queryClient.invalidateQueries({ queryKey: [key, userId, portfolioId] }),
      ),
    );
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
        etag: readEtag(result.response, flowEtag(latest)),
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
      const current = await getCapitalFlow({ path: { portfolioId, flowId } });
      const latest = unwrapOrRedirect(current, redirect);
      await refreshCsrf();
      const result = await deleteCapitalFlow({
        path: { portfolioId, flowId },
        headers: { "If-Match": readEtag(current.response, flowEtag(latest)) },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setError("The flow changed in another session. Reload the list and retry.");
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
    <section className={styles.wide}>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      ) : null}
      <section className={styles.panel}>
        <div className={styles.panelHeader}>
          <div>
            <h2>Capital Flows</h2>
            <p className={styles.muted} style={{ margin: "2px 0 0" }}>
              Manual contributions and withdrawals recorded to adjust portfolio valuation.
            </p>
          </div>
          <ActionButton type="button" tone="primary" disabled={busy} onClick={openCreate}>
            Record flow
          </ActionButton>
        </div>
        <form className={styles.compactFilterBar} onSubmit={(event) => event.preventDefault()}>
          <label className={styles.compactFilterField}>
            <span className={styles.compactFilterLabel}>From</span>
            <TextInput
              type="datetime-local"
              value={from}
              className={styles.compactDateInput}
              onChange={(event) => update({ from: event.target.value })}
            />
          </label>
          <label className={styles.compactFilterField}>
            <span className={styles.compactFilterLabel}>To</span>
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
        <QueryRecovery
          error={flows.error}
          hasData={Boolean(flows.data)}
          busy={flows.isFetching}
          fallback="Capital Flows could not be loaded."
          onRetry={() => {
            void flows.refetch();
          }}
        />
        <DataTable
          caption="Manual Capital Flows, newest first"
          columns={[
            { key: "effectiveAt", header: "Effective at", nowrap: true },
            { key: "direction", header: "Direction" },
            { key: "assetSymbol", header: "Asset" },
            { key: "quantity", header: "Quantity", numeric: true },
            { key: "valueUsd", header: "Value (USD)", numeric: true },
            { key: "account", header: "Account" },
            { key: "note", header: "Note" },
            { key: "actions", header: "" },
          ]}
          rows={(flows.data?.items ?? []).map((item) => {
            const dec = parseDecimal(item.valueUsd);
            const isContribution = item.direction.toLowerCase() === "contribution";
            return {
              id: item.id,
              effectiveAt: formatInstant(item.effectiveAt, timezone),
              direction: <DirectionBadge value={item.direction} />,
              assetSymbol: item.assetSymbol,
              quantity: formatDecimal(item.quantity),
              valueUsd: dec ? (
                <span className={isContribution ? styles.pnlPositive : styles.pnlNegative}>
                  <NumericText>{`${isContribution ? "+" : "-"}$${formatAmount(dec.abs().toFixed())}`}</NumericText>
                </span>
              ) : (
                <span className={styles.muted}>—</span>
              ),
              account:
                item.accountName ??
                (item.accountId ? item.accountId : <span className={styles.muted}>—</span>),
              note: item.note ?? "—",
              actions: (
                <span className={styles.rowActions}>
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
      <section className={styles.panel}>
        <div className={styles.panelHeader}>
          <div>
            <h2>Boundary Events</h2>
            <p className={styles.muted} style={{ margin: "2px 0 0" }}>
              Automatic adjustments recorded when accounts enter or leave the active boundary.
            </p>
          </div>
        </div>
        <form className={styles.compactFilterBar} onSubmit={(event) => event.preventDefault()}>
          <label className={styles.compactFilterField}>
            <span className={styles.compactFilterLabel}>From</span>
            <TextInput
              type="datetime-local"
              value={eventsFrom}
              className={styles.compactDateInput}
              onChange={(event) => update({ eventsFrom: event.target.value })}
            />
          </label>
          <label className={styles.compactFilterField}>
            <span className={styles.compactFilterLabel}>To</span>
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
        <QueryRecovery
          error={events.error}
          hasData={Boolean(events.data)}
          busy={events.isFetching}
          fallback="Boundary Events could not be loaded."
          onRetry={() => {
            void events.refetch();
          }}
        />
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
        <p className={styles.lead}>
          <NumericText>
            {deleteTarget
              ? `Delete the ${deleteTarget.direction} of ${formatDecimal(deleteTarget.quantity)} ${deleteTarget.assetSymbol} effective ${formatInstant(deleteTarget.effectiveAt, timezone)}? Value Changes over that period will no longer account for it.`
              : ""}
          </NumericText>
        </p>
        <div className={styles.actions}>
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
