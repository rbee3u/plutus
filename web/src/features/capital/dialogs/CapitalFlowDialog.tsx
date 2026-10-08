import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useRef, useState } from "react";
import {
  createCapitalFlow,
  getCapitalFlow,
  patchCapitalFlow,
  problemMessage,
  readEtag,
} from "@/api";
import type { CapitalFlow } from "@/api";
import type { AccountOption } from "@/shared/lib/useIdentities";
import { fromDatetimeLocalValue, toDatetimeLocalValue } from "@/shared/lib/datetime";
import { newIdempotencyKey } from "@/shared/lib/id";
import { parseDecimal } from "@/shared/lib/amount";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { Select, TextArea, TextInput } from "@/shared/ui/FormControls";
import { SegmentedControl } from "@/shared/ui/SegmentedControl";
import styles from "@/features/capital/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function flowEtag(item: CapitalFlow): string {
  return `"capital-flow:${String(item.version)}"`;
}

export type FlowDialogRequest =
  | { nonce: number; mode: "create" }
  | { nonce: number; mode: "edit"; item: CapitalFlow; etag: string };

interface FlowFields {
  direction: "contribution" | "withdrawal";
  assetSymbol: string;
  quantity: string;
  valueUsd: string;
  effectiveAt: string;
  accountId: string;
  note: string;
}

const emptyForm: FlowFields = {
  direction: "contribution",
  assetSymbol: "",
  quantity: "",
  valueUsd: "",
  effectiveAt: "",
  accountId: "",
  note: "",
};

function formFromFlow(item: CapitalFlow): FlowFields {
  return {
    direction: item.direction,
    assetSymbol: item.assetSymbol,
    quantity: item.quantity,
    valueUsd: item.valueUsd ?? "",
    effectiveAt: toDatetimeLocalValue(item.effectiveAt),
    accountId: item.accountId ?? "",
    note: item.note ?? "",
  };
}

export function CapitalFlowDialog({
  portfolioId,
  request,
  accounts,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  request: FlowDialogRequest | null;
  accounts: AccountOption[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  return (
    <Dialog
      open={request !== null}
      title={request?.mode === "edit" ? "Edit Capital Flow" : "Record Capital Flow"}
      onClose={onClose}
    >
      {request ? (
        <FlowForm
          key={request.nonce}
          portfolioId={portfolioId}
          request={request}
          accounts={accounts}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
    </Dialog>
  );
}

function FlowForm({
  portfolioId,
  request,
  accounts,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  request: FlowDialogRequest;
  accounts: AccountOption[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  const { redirect } = useSessionScope();
  const write = useAuthenticatedWrite();
  const createKey = useRef(newIdempotencyKey());
  const [form, setForm] = useState<FlowFields>(() =>
    request.mode === "edit" ? formFromFlow(request.item) : emptyForm,
  );
  const [etag, setEtag] = useState(request.mode === "edit" ? request.etag : "");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const editingId = request.mode === "edit" ? request.item.id : null;
  const knownAccount = !form.accountId || accounts.some((account) => account.id === form.accountId);

  function set(patch: Partial<FlowFields>) {
    setForm((current) => ({ ...current, ...patch }));
  }

  async function reloadLatest(flowId: string, message: string) {
    const result = await getCapitalFlow({ path: { portfolioId, flowId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      setForm(formFromFlow(result.data));
      setEtag(readEtag(result.response, flowEtag(result.data)));
    }
    setError(message);
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const effectiveAt = fromDatetimeLocalValue(form.effectiveAt);
      const assetSymbol = form.assetSymbol.trim();
      const quantity = form.quantity.trim();
      const valueUsd = form.valueUsd.trim();
      const note = form.note.trim();
      // The server answers every malformed field with one generic problem; name the field here.
      const quantityValue = parseDecimal(quantity);
      if (quantityValue === null || quantityValue.lte(0)) {
        setError("Quantity must be a positive number.");
        return;
      }
      const usdValue = valueUsd ? parseDecimal(valueUsd) : null;
      if (valueUsd && (usdValue === null || usdValue.lt(0))) {
        setError("Value (USD) must be zero or more, or left empty when unknown.");
        return;
      }
      if (editingId) {
        const result = await write(() =>
          patchCapitalFlow({
            path: { portfolioId, flowId: editingId },
            headers: { "If-Match": etag },
            body: {
              direction: form.direction,
              assetSymbol,
              quantity,
              valueUsd: valueUsd ? valueUsd : null,
              effectiveAt,
              accountId: form.accountId ? form.accountId : null,
              note: note ? note : null,
            },
          }),
        );
        if (result.response?.status === 412) {
          await reloadLatest(
            editingId,
            "The flow changed in another session. The form now shows the latest values; review and save again.",
          );
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The Capital Flow could not be updated."));
          return;
        }
        await onSaved("Capital Flow updated.");
      } else {
        const result = await write(() =>
          createCapitalFlow({
            path: { portfolioId },
            headers: { "Idempotency-Key": createKey.current },
            body: {
              direction: form.direction,
              assetSymbol,
              quantity,
              effectiveAt,
              ...(valueUsd ? { valueUsd } : {}),
              ...(form.accountId ? { accountId: form.accountId } : {}),
              ...(note ? { note } : {}),
            },
          }),
        );
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The Capital Flow could not be recorded."));
          return;
        }
        createKey.current = newIdempotencyKey();
        await onSaved("Capital Flow recorded.");
      }
    } catch (cause) {
      setError(problemMessage(cause, "The Capital Flow could not be saved."));
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
      <div className={shared.field}>
        <span>Direction</span>
        <SegmentedControl<"contribution" | "withdrawal">
          value={form.direction}
          onChange={(direction) => set({ direction })}
          label="Direction"
          options={[
            { value: "contribution", label: "+ Contribution" },
            { value: "withdrawal", label: "- Withdrawal" },
          ]}
        />
      </div>
      <div className={styles.formRow2}>
        <label className={shared.field}>
          <span>Asset symbol</span>
          <TextInput
            value={form.assetSymbol}
            placeholder="USDT"
            onChange={(event) => set({ assetSymbol: event.target.value })}
            required
          />
        </label>
        <label className={shared.field}>
          <span>Quantity</span>
          <TextInput
            value={form.quantity}
            inputMode="decimal"
            placeholder="0.00"
            onChange={(event) => set({ quantity: event.target.value })}
            required
          />
        </label>
      </div>
      <div className={styles.formRow2}>
        <label className={shared.field}>
          <span>Effective at</span>
          <TextInput
            type="datetime-local"
            value={form.effectiveAt}
            onChange={(event) => set({ effectiveAt: event.target.value })}
            required
          />
        </label>
        <label className={shared.field}>
          <span>Value (USD, optional)</span>
          <TextInput
            value={form.valueUsd}
            inputMode="decimal"
            placeholder="Leave empty when unknown"
            onChange={(event) => set({ valueUsd: event.target.value })}
          />
        </label>
      </div>
      <label className={shared.field}>
        <span>Account (optional)</span>
        <Select value={form.accountId} onChange={(event) => set({ accountId: event.target.value })}>
          <option value="">Portfolio level</option>
          {!knownAccount ? <option value={form.accountId}>Current account</option> : null}
          {accounts.map((account) => (
            <option key={account.id} value={account.id}>
              {account.label}
            </option>
          ))}
        </Select>
      </label>
      <label className={shared.field}>
        <span>Note</span>
        <TextArea
          value={form.note}
          rows={2}
          placeholder="Optional notes"
          onChange={(event) => set({ note: event.target.value })}
        />
      </label>
      <div className={shared.actions}>
        <ActionButton type="submit" tone="primary" disabled={busy}>
          {editingId ? "Save flow" : "Record flow"}
        </ActionButton>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
      </div>
    </form>
  );
}
