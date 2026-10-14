import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  deleteCapitalFlow,
  getCapitalFlow,
  invalidatePortfolioReads,
  keys,
  listCapitalFlows,
  problemMessage,
  unwrapOrRedirect,
} from "@/api";
import type { CapitalFlow, CapitalFlowList } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatAmount, formatCount, formatDecimal, parseDecimal } from "@/shared/lib/amount";
import { formatInstant, isoFromDateInput, normalizeDateInput } from "@/shared/lib/datetime";
import { useSearchState } from "@/shared/lib/searchParams";
import { useSessionScope } from "@/shared/lib/session";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { accountOptions, useIdentities } from "@/shared/lib/useIdentities";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DirectionBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { TextInput } from "@/shared/ui/FormControls";
import { NumericText } from "@/shared/ui/NumericText";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { FlowActions } from "./FlowActions";
import { CapitalFlowDialog, type FlowDialogRequest, flowEtag } from "../dialogs/CapitalFlowDialog";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function CapitalFlowsPanel({
  portfolioId,
  timezone,
}: {
  portfolioId: string;
  timezone?: string;
}) {
  const { userId, redirect } = useSessionScope();
  const write = useAuthenticatedWrite();
  const queryClient = useQueryClient();
  const { params, update } = useSearchState();
  const rawFrom = params.get("from");
  const rawTo = params.get("to");
  const from = normalizeDateInput(rawFrom);
  const to = normalizeDateInput(rawTo);

  useEffect(() => {
    if ((rawFrom !== null && rawFrom !== from) || (rawTo !== null && rawTo !== to)) {
      update({ from, to });
    }
  }, [rawFrom, rawTo, from, to, update]);

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
        query: { from: isoFromDateInput(from), to: isoFromDateInput(to, "exclusiveEnd") },
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

  const items = flows.data?.items ?? [];
  const hasFilter = Boolean(from || to);

  return (
    <>
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
      <QueryRecovery query={flows} fallback="Capital Flows could not be loaded." />
      <div className={`${shared.tableToolbar} ${styles.tableToolbar}`}>
        <div className={`${shared.summaryBar} ${styles.summaryBar}`}>
          <span className={shared.summaryCount}>
            {formatCount(items.length)} {items.length === 1 ? "flow" : "flows"}
          </span>
          {hasFilter ? (
            <ActionButton type="button" tone="ghost" onClick={() => update({ from: "", to: "" })}>
              Clear filters
            </ActionButton>
          ) : null}
        </div>
        <div className={`${shared.toolbarActions} ${styles.toolbarActions}`}>
          <form
            className={`${shared.compactFilterBar} ${styles.filterForm}`}
            onSubmit={(event) => event.preventDefault()}
          >
            <label className={`${shared.compactFilterField} ${styles.filterField}`}>
              <span className={shared.compactFilterLabel}>From</span>
              <TextInput
                type="date"
                value={from}
                className={styles.compactDateInput}
                onChange={(event) => update({ from: event.target.value })}
              />
            </label>
            <label className={`${shared.compactFilterField} ${styles.filterField}`}>
              <span className={shared.compactFilterLabel}>To</span>
              <TextInput
                type="date"
                value={to}
                className={styles.compactDateInput}
                onChange={(event) => update({ to: event.target.value })}
              />
            </label>
          </form>
          <ActionButton
            type="button"
            tone="primary"
            className={styles.primaryAction}
            disabled={busy}
            onClick={openCreate}
          >
            <Plus size={iconSize.sm} aria-hidden="true" />
            <span>Record flow</span>
          </ActionButton>
        </div>
      </div>
      <DataTable
        caption="Manual Capital Flows, newest first"
        columns={[
          { key: "effectiveAt", header: "Effective at", nowrap: true, width: "170px" },
          { key: "direction", header: "Direction", width: "110px" },
          { key: "canonicalAssetSymbol", header: "Asset", width: "120px" },
          { key: "quantity", header: "Quantity", numeric: true, width: "130px" },
          { key: "valueUsd", header: "Value (USD)", numeric: true, width: "140px" },
          { key: "account", header: "Account", width: "150px" },
          { key: "note", header: "Note" },
          { key: "actions", header: "", width: "60px" },
        ]}
        rows={items.map((item) => {
          const dec = parseDecimal(item.valueUsd);
          return {
            id: item.id,
            effectiveAt: (
              <span className={shared.tabularDate}>
                {formatInstant(item.effectiveAt, timezone)}
              </span>
            ),
            direction: <DirectionBadge value={item.direction} />,
            canonicalAssetSymbol: <strong>{item.canonicalAssetSymbol}</strong>,
            quantity: <NumericText>{formatDecimal(item.quantity)}</NumericText>,
            valueUsd: dec ? (
              <NumericText>{formatAmount(dec.abs().toFixed())}</NumericText>
            ) : (
              <span className={shared.muted}>—</span>
            ),
            account:
              item.accountName ??
              (item.accountId ? item.accountId : <span className={shared.muted}>—</span>),
            note: item.note ?? "—",
            actions: (
              <FlowActions
                item={item}
                disabled={busy}
                onEdit={(target) => void openEdit(target)}
                onDelete={(target) => setDeleteTarget(target)}
              />
            ),
          };
        })}
        empty={
          flows.isPending ? (
            <TableEmptyState title="Loading Capital Flows..." />
          ) : hasFilter ? (
            <TableEmptyState
              title="No Capital Flows match this date range"
              help="Try adjusting your from or to dates."
              action={
                <ActionButton
                  type="button"
                  tone="secondary"
                  onClick={() => update({ from: "", to: "" })}
                >
                  Clear date filter
                </ActionButton>
              }
            />
          ) : (
            <TableEmptyState
              title="No Capital Flows recorded yet"
              help="Record external contributions or withdrawals to track capital flows."
              action={
                <ActionButton tone="primary" onClick={openCreate}>
                  <Plus size={iconSize.sm} aria-hidden="true" /> Record flow
                </ActionButton>
              }
            />
          )
        }
      />
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
        {deleteTarget ? (
          <>
            <p className={shared.lead}>
              Are you sure you want to delete this {deleteTarget.direction}?
            </p>
            <div className={styles.flowSummaryCard}>
              <div className={styles.flowSummaryItem}>
                <span className={styles.flowSummaryLabel}>Amount</span>
                <span className={styles.flowSummaryValue}>
                  <NumericText>{`${formatDecimal(deleteTarget.quantity)} ${deleteTarget.canonicalAssetSymbol}`}</NumericText>
                </span>
              </div>
              <div className={styles.flowSummaryItem}>
                <span className={styles.flowSummaryLabel}>Effective</span>
                <span className={styles.flowSummaryValue}>
                  {formatInstant(deleteTarget.effectiveAt, timezone)}
                </span>
              </div>
              {deleteTarget.accountName ? (
                <div className={styles.flowSummaryItem}>
                  <span className={styles.flowSummaryLabel}>Account</span>
                  <span className={styles.flowSummaryValue}>{deleteTarget.accountName}</span>
                </div>
              ) : null}
            </div>
            <div className={shared.dialogActions}>
              <ActionButton
                type="button"
                tone="ghost"
                disabled={busy}
                onClick={() => setDeleteTarget(null)}
              >
                Cancel
              </ActionButton>
              <ActionButton
                type="button"
                tone="danger"
                disabled={busy}
                onClick={() => void confirmDelete()}
              >
                {busy ? "Deleting..." : "Delete"}
              </ActionButton>
            </div>
          </>
        ) : null}
      </Dialog>
    </>
  );
}
