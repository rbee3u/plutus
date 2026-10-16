import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useId, useRef, useState } from "react";
import { createCapitalFlow, getCapitalFlow, patchCapitalFlow, problemMessage } from "@/api";
import { usePlatforms } from "@/shared/lib/usePlatforms";
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

export type FlowDialogRequest =
  { nonce: number; mode: "create" } | { nonce: number; mode: "edit"; item: CapitalFlow };

interface FlowFields {
  direction: "contribution" | "withdrawal";
  canonicalAssetSymbol: string;
  quantity: string;
  valueUsd: string;
  effectiveAt: string;
  accountId: string;
  pricingPlatform: string;
  note: string;
}

function newEmptyForm(timezone: string): FlowFields {
  return {
    direction: "contribution",
    canonicalAssetSymbol: "",
    quantity: "",
    valueUsd: "",
    effectiveAt: toDatetimeLocalValue(new Date().toISOString(), timezone),
    accountId: "",
    pricingPlatform: "",
    note: "",
  };
}

function formFromFlow(item: CapitalFlow, timezone: string): FlowFields {
  return {
    direction: item.direction,
    canonicalAssetSymbol: item.canonicalAssetSymbol,
    quantity: item.quantity,
    valueUsd: item.pricingMethod === "explicit" ? (item.valueUsd ?? "") : "",
    effectiveAt: toDatetimeLocalValue(item.effectiveAt, timezone),
    accountId: item.accountId ?? "",
    pricingPlatform: item.pricingPlatform ?? "",
    note: item.note ?? "",
  };
}

export function CapitalFlowDialog({
  portfolioId,
  timezone,
  request,
  accounts,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  timezone?: string;
  request: FlowDialogRequest | null;
  accounts: AccountOption[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  return (
    <Dialog
      open={request !== null}
      title={request?.mode === "edit" ? "Edit capital flow" : "Record capital flow"}
      onClose={onClose}
    >
      {request && timezone ? (
        <FlowForm
          key={request.nonce}
          portfolioId={portfolioId}
          timezone={timezone}
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
  timezone: initialTimezone,
  request,
  accounts,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  timezone: string;
  request: FlowDialogRequest;
  accounts: AccountOption[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}) {
  // Keep the wall-time interpretation with the draft if portfolio reads refresh.
  const [timezone] = useState(initialTimezone);
  const { redirect } = useSessionScope();
  const platforms = usePlatforms();
  const write = useAuthenticatedWrite();
  const valueId = useId();
  const effectiveAtId = useId();
  const createKey = useRef(newIdempotencyKey());
  const [form, setForm] = useState<FlowFields>(() =>
    request.mode === "edit" ? formFromFlow(request.item, timezone) : newEmptyForm(timezone),
  );
  const [baseline, setBaseline] = useState(request.mode === "edit" ? request.item : null);
  const [needsReload, setNeedsReload] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const editingId = request.mode === "edit" ? request.item.id : null;
  const knownAccount = !form.accountId || accounts.some((account) => account.id === form.accountId);

  function set(patch: Partial<FlowFields>) {
    setForm((current) => ({ ...current, ...patch }));
  }

  async function reloadLatest() {
    if (!baseline) return;
    setBusy(true);
    try {
      const result = await getCapitalFlow({ path: { portfolioId, flowId: baseline.id } });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.error || !result.data) {
        throw new Error(
          problemMessage(result.error, "The latest capital flow could not be loaded."),
        );
      }
      setForm(formFromFlow(result.data, timezone));
      setBaseline(result.data);
      setNeedsReload(false);
      setError(null);
    } catch (cause) {
      setError(problemMessage(cause, "The latest capital flow could not be loaded."));
    } finally {
      setBusy(false);
    }
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (busy || needsReload) return;
    setBusy(true);
    setError(null);
    try {
      const timeChanged =
        !baseline || form.effectiveAt !== toDatetimeLocalValue(baseline.effectiveAt, timezone);
      const effectiveAt = timeChanged
        ? fromDatetimeLocalValue(form.effectiveAt, timezone)
        : baseline.effectiveAt;
      const canonicalAssetSymbol = form.canonicalAssetSymbol.trim();
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
      if (baseline) {
        const result = await write(() =>
          patchCapitalFlow({
            path: { portfolioId, flowId: baseline.id },
            headers: { "If-Edit-Version": String(baseline.version) },
            body: {
              ...(form.direction !== baseline.direction ? { direction: form.direction } : {}),
              ...(canonicalAssetSymbol !== baseline.canonicalAssetSymbol
                ? { canonicalAssetSymbol }
                : {}),
              ...(quantity !== baseline.quantity ? { quantity } : {}),
              ...(valueUsd !== formFromFlow(baseline, timezone).valueUsd
                ? { valueUsd: valueUsd || null }
                : {}),
              ...(timeChanged ? { effectiveAt } : {}),
              ...(form.accountId !== (baseline.accountId ?? "")
                ? { accountId: form.accountId || null }
                : {}),
              ...(!form.accountId &&
              (form.accountId !== (baseline.accountId ?? "") ||
                form.pricingPlatform !== (baseline.pricingPlatform ?? ""))
                ? { pricingPlatform: form.pricingPlatform || null }
                : {}),
              ...(note !== (baseline.note ?? "") ? { note: note || null } : {}),
            },
          }),
        );
        if (result.response?.status === 412) {
          setNeedsReload(true);
          setError(
            "The flow changed in another session. Reload the latest flow to review before saving again.",
          );
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The capital flow could not be updated."));
          return;
        }
        await onSaved("Capital flow updated.");
      } else {
        const result = await write(() =>
          createCapitalFlow({
            path: { portfolioId },
            headers: { "Idempotency-Key": createKey.current },
            body: {
              direction: form.direction,
              canonicalAssetSymbol,
              quantity,
              effectiveAt,
              ...(valueUsd ? { valueUsd } : {}),
              ...(form.accountId ? { accountId: form.accountId } : {}),
              ...(!form.accountId && form.pricingPlatform
                ? { pricingPlatform: form.pricingPlatform }
                : {}),
              ...(note ? { note } : {}),
            },
          }),
        );
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The capital flow could not be recorded."));
          return;
        }
        createKey.current = newIdempotencyKey();
        await onSaved("Capital flow recorded.");
      }
    } catch (cause) {
      setError(problemMessage(cause, "The capital flow could not be saved."));
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
      {needsReload ? (
        <ActionButton type="button" disabled={busy} onClick={() => void reloadLatest()}>
          Reload latest flow
        </ActionButton>
      ) : null}
      <div className={shared.field}>
        <span>Direction</span>
        <SegmentedControl<"contribution" | "withdrawal">
          value={form.direction}
          onChange={(direction) => set({ direction })}
          label="Direction"
          options={[
            { value: "contribution", label: "+ Contribution", disabled: busy },
            { value: "withdrawal", label: "- Withdrawal", disabled: busy },
          ]}
        />
      </div>
      <div className={styles.formRow2}>
        <label className={shared.field}>
          <span>Asset</span>
          <TextInput
            disabled={busy}
            value={form.canonicalAssetSymbol}
            placeholder="USDT"
            autoCapitalize="characters"
            autoCorrect="off"
            spellCheck={false}
            autoFocus={request.mode === "create"}
            style={{
              fontFamily: "var(--font-mono)",
              fontWeight: "var(--weight-semibold)",
              letterSpacing: "0.04em",
            }}
            onChange={(event) => set({ canonicalAssetSymbol: event.target.value.toUpperCase() })}
            required
          />
        </label>
        <label className={shared.field}>
          <span>Quantity</span>
          <TextInput
            disabled={busy}
            value={form.quantity}
            inputMode="decimal"
            placeholder="0.00"
            style={{ fontFamily: "var(--font-mono)" }}
            onChange={(event) => set({ quantity: event.target.value })}
            required
          />
        </label>
      </div>
      <div className={styles.formRow2}>
        <div className={shared.field}>
          <label htmlFor={effectiveAtId}>Effective at</label>
          <TextInput
            id={effectiveAtId}
            disabled={busy}
            type="datetime-local"
            step="1"
            aria-describedby={`${valueId}-timezone`}
            value={form.effectiveAt}
            onChange={(event) => set({ effectiveAt: event.target.value })}
            required
          />
          <span id={`${valueId}-timezone`} className={shared.muted}>
            Timezone: {timezone}
          </span>
        </div>
        <div className={shared.field}>
          <label htmlFor={valueId}>Value (USD, optional)</label>
          <TextInput
            id={valueId}
            aria-describedby={`${valueId}-hint`}
            disabled={busy}
            value={form.valueUsd}
            inputMode="decimal"
            style={{ fontFamily: "var(--font-mono)" }}
            onChange={(event) => set({ valueUsd: event.target.value })}
          />
          <span id={`${valueId}-hint`} className={shared.muted}>
            Calculated automatically when possible; otherwise unknown.
          </span>
        </div>
      </div>
      <label className={shared.field}>
        <span>Account (optional)</span>
        <Select
          disabled={busy}
          value={form.accountId}
          onChange={(event) => set({ accountId: event.target.value })}
        >
          <option value="">Portfolio level</option>
          {!knownAccount ? <option value={form.accountId}>Current account</option> : null}
          {accounts.map((account) => (
            <option key={account.id} value={account.id}>
              {account.label}
            </option>
          ))}
        </Select>
      </label>
      {!form.accountId && !form.valueUsd ? (
        <label className={shared.field}>
          <span>Pricing platform (optional)</span>
          <Select
            disabled={busy}
            value={form.pricingPlatform}
            onChange={(event) => set({ pricingPlatform: event.target.value })}
          >
            <option value="">None</option>
            {platforms.data?.items.map((platform) => (
              <option key={platform.id} value={platform.id}>
                {platform.name}
              </option>
            ))}
          </Select>
          <span className={shared.muted}>Used to price assets when no account is specified.</span>
        </label>
      ) : null}
      <label className={shared.field}>
        <span>Note</span>
        <TextArea
          disabled={busy}
          value={form.note}
          rows={2}
          maxLength={500}
          placeholder="Optional notes"
          onChange={(event) => set({ note: event.target.value })}
        />
      </label>
      <div className={shared.dialogActions}>
        <ActionButton type="button" tone="ghost" disabled={busy} onClick={onClose}>
          Cancel
        </ActionButton>
        <ActionButton type="submit" tone="primary" disabled={busy || needsReload}>
          {editingId ? "Save flow" : "Record flow"}
        </ActionButton>
      </div>
    </form>
  );
}
