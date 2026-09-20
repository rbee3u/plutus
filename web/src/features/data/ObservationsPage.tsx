import { type FormEvent, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getObservation,
  getOverview,
  listAccounts,
  listIdentities,
  listObservations,
  patchObservation,
} from "@/api/generated/sdk.gen";
import type {
  Observation,
  ObservationKind,
  PatchObservationRequest,
} from "@/api/generated/types.gen";
import { safeJson } from "@/features/data/json";
import { useProjectionFollow } from "@/features/data/useProjectionFollow";
import { problemMessage, refreshCsrf } from "@/api/http";
import { formatInstant, isoFromLocalInput } from "@/shared/lib/datetime";
import { readEtag, unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { CursorPager } from "@/shared/ui/Pager";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;
const KINDS = ["", "balance", "position", "reported_equity", "account_metric", "price"] as const;

export function ObservationsPage() {
  const { portfolioId = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const kind = readKind(params.get("kind"));
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const from = params.get("from") ?? "";
  const to = params.get("to") ?? "";

  function update(next: Record<string, string>) {
    const merged = new URLSearchParams(params);
    for (const [key, value] of Object.entries(next)) {
      if (value) merged.set(key, value);
      else merged.delete(key);
    }
    setParams(merged, { replace: true });
  }

  return (
    <section className={styles.wide}>
      <h2>Observations</h2>
      <p className={styles.lead}>
        Search normalized facts and correct a typed Observation. Corrections audit before/after and
        enqueue a projection rebuild.
      </p>
      <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
        <label className={styles.field}>
          <span>Kind</span>
          <Select value={kind} onChange={(event) => update({ kind: event.target.value })}>
            {KINDS.map((option) => (
              <option key={option || "all"} value={option}>
                {option || "All"}
              </option>
            ))}
          </Select>
        </label>
        <IdentityAccountFilters
          portfolioId={portfolioId}
          identityId={identityId}
          accountId={accountId}
          onIdentity={(value) => update({ identityId: value })}
          onAccount={(value) => update({ accountId: value })}
        />
        <label className={styles.field}>
          <span>From</span>
          <TextInput
            type="datetime-local"
            value={from}
            onChange={(event) => update({ from: event.target.value })}
          />
        </label>
        <label className={styles.field}>
          <span>To</span>
          <TextInput
            type="datetime-local"
            value={to}
            onChange={(event) => update({ to: event.target.value })}
          />
        </label>
      </form>
      <ObservationResults
        key={`${kind}|${identityId}|${accountId}|${from}|${to}`}
        portfolioId={portfolioId}
        kind={kind}
        identityId={identityId}
        accountId={accountId}
        from={from}
        to={to}
      />
    </section>
  );
}

function IdentityAccountFilters({
  portfolioId,
  identityId,
  accountId,
  onIdentity,
  onAccount,
}: {
  portfolioId: string;
  identityId: string;
  accountId: string;
  onIdentity: (value: string) => void;
  onAccount: (value: string) => void;
}) {
  const navigate = useNavigate();
  const user = useCurrentUser();
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const identities = useQuery({
    queryKey: ["identities", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await listIdentities({ path: { portfolioId } }), redirect),
  });
  const accounts = useQuery({
    queryKey: ["accounts", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAccounts({ path: { portfolioId }, query: { limit: 200 } }),
        redirect,
      ),
  });
  return (
    <>
      <label className={styles.field}>
        <span>Identity</span>
        <Select value={identityId} onChange={(event) => onIdentity(event.target.value)}>
          <option value="">All</option>
          {(identities.data?.items ?? []).map((item) => (
            <option key={item.id} value={item.id}>
              {item.displayName}
            </option>
          ))}
        </Select>
      </label>
      <label className={styles.field}>
        <span>Account</span>
        <Select value={accountId} onChange={(event) => onAccount(event.target.value)}>
          <option value="">All</option>
          {(accounts.data?.items ?? []).map((item) => (
            <option key={item.id} value={item.id}>
              {item.displayName}
            </option>
          ))}
        </Select>
      </label>
    </>
  );
}

function ObservationResults({
  portfolioId,
  kind,
  identityId,
  accountId,
  from,
  to,
}: {
  portfolioId: string;
  kind: ObservationKind | "";
  identityId: string;
  accountId: string;
  from: string;
  to: string;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const follow = useProjectionFollow(portfolioId);
  const [cursors, setCursors] = useState<string[]>([""]);
  const [detail, setDetail] = useState<Observation | null>(null);
  const [etag, setEtag] = useState("");
  const [form, setForm] = useState<PatchObservationRequest>({});
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const cursor = cursors[cursors.length - 1] ?? "";
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const query = useQuery({
    queryKey: [
      "observations",
      user.data?.id,
      portfolioId,
      kind,
      identityId,
      accountId,
      from,
      to,
      cursor,
    ],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listObservations({
          path: { portfolioId },
          query: {
            kind: kind || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
            from: isoFromLocalInput(from),
            to: isoFromLocalInput(to),
            limit: PAGE_SIZE,
            cursor: cursor || undefined,
          },
        }),
        redirect,
      ),
  });

  function applyObservation(item: Observation, nextEtag: string) {
    setDetail(item);
    setEtag(nextEtag);
    setForm(formFromObservation(item));
  }

  async function openDetail(observationId: string) {
    setError(null);
    const result = await getObservation({ path: { portfolioId, observationId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    const item = unwrapOrRedirect(result, redirect);
    applyObservation(item, readEtag(result.response, `"observation:${String(item.version)}"`));
  }

  async function loadLatest(observationId: string, message: string) {
    const result = await getObservation({ path: { portfolioId, observationId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      applyObservation(
        result.data,
        readEtag(result.response, `"observation:${String(result.data.version)}"`),
      );
    }
    setError(message);
  }

  async function onCorrect(event: FormEvent) {
    event.preventDefault();
    if (!detail) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const overview = await queryClient.fetchQuery({
        queryKey: ["overview", user.data?.id, portfolioId],
        queryFn: async () =>
          unwrapOrRedirect(await getOverview({ path: { portfolioId } }), redirect),
      });
      await refreshCsrf();
      const result = await patchObservation({
        path: { portfolioId, observationId: detail.id },
        headers: { "If-Match": etag },
        body: form,
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await loadLatest(
          detail.id,
          "The observation changed in another session. The form now shows the latest server values.",
        );
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The observation could not be corrected."));
        return;
      }
      applyObservation(
        result.data.observation,
        readEtag(result.response, `"observation:${String(result.data.observation.version)}"`),
      );
      follow.follow(result.data.job, overview.context.generationId);
      await queryClient.invalidateQueries({
        queryKey: ["observations", user.data?.id, portfolioId],
      });
    } catch (cause) {
      setError(problemMessage(cause, "The observation could not be corrected."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {query.error ? (
        <p className={styles.alert} role="alert">
          {query.error instanceof Error ? query.error.message : "Observations could not be loaded."}
        </p>
      ) : null}
      {follow.notice ? <p className={styles.notice}>{follow.notice}</p> : null}
      <DataTable
        caption="Normalized observations"
        columns={[
          { key: "kind", header: "Kind" },
          { key: "summary", header: "Summary" },
          { key: "completeness", header: "Completeness" },
          { key: "observed", header: "Source observed" },
          { key: "collected", header: "Collected" },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          kind: item.kind,
          summary: item.summary ?? "—",
          completeness: item.completeness,
          observed: formatInstant(item.sourceObservedAt),
          collected: formatInstant(item.collectedAt),
        }))}
      />
      <ul className={styles.flowActions}>
        {(query.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>
              {item.kind} {item.summary ?? item.id}
            </span>
            <span className={styles.actions}>
              <ActionButton type="button" disabled={busy} onClick={() => void openDetail(item.id)}>
                Open
              </ActionButton>
            </span>
          </li>
        ))}
      </ul>
      <CursorPager
        canPrev={cursors.length > 1}
        hasMore={query.data?.page.hasMore ?? false}
        onPrev={() => setCursors((items) => items.slice(0, -1))}
        onNext={() => {
          const next = query.data?.page.nextCursor;
          if (next) setCursors((items) => [...items, next]);
        }}
      />
      <Dialog
        open={Boolean(detail)}
        title="Observation"
        onClose={() => {
          setDetail(null);
          setError(null);
        }}
      >
        {detail ? (
          <form className={styles.form} onSubmit={(event) => void onCorrect(event)}>
            {error ? (
              <p className={styles.alert} role="alert">
                {error}
              </p>
            ) : null}
            <p className={styles.muted}>
              {detail.kind} · version {String(detail.version)} · {detail.completeness}
            </p>
            <pre className={styles.jsonBlock}>{safeJson(typedPayload(detail))}</pre>
            <CorrectionFields kind={detail.kind} form={form} onChange={setForm} />
            <div className={styles.actions}>
              <ActionButton type="submit" tone="primary" disabled={busy}>
                Save correction
              </ActionButton>
              <ActionButton type="button" onClick={() => setDetail(null)}>
                Close
              </ActionButton>
            </div>
          </form>
        ) : null}
      </Dialog>
    </>
  );
}

function CorrectionFields({
  kind,
  form,
  onChange,
}: {
  kind: ObservationKind;
  form: PatchObservationRequest;
  onChange: (next: PatchObservationRequest) => void;
}) {
  const set = (patch: PatchObservationRequest) => onChange({ ...form, ...patch });
  return (
    <>
      <label className={styles.field}>
        <span>Completeness</span>
        <Select
          value={form.completeness ?? ""}
          onChange={(event) =>
            set({
              completeness:
                event.target.value === "complete" || event.target.value === "partial"
                  ? event.target.value
                  : undefined,
            })
          }
        >
          <option value="complete">complete</option>
          <option value="partial">partial</option>
        </Select>
      </label>
      {kind === "balance" ? (
        <>
          <Field
            label="Total quantity"
            value={form.totalQuantity ?? ""}
            onChange={(value) => set({ totalQuantity: emptyToNull(value) })}
          />
          <Field
            label="Available quantity"
            value={form.availableQuantity ?? ""}
            onChange={(value) => set({ availableQuantity: emptyToNull(value) })}
          />
          <Field
            label="Locked quantity"
            value={form.lockedQuantity ?? ""}
            onChange={(value) => set({ lockedQuantity: emptyToNull(value) })}
          />
        </>
      ) : null}
      {kind === "position" ? (
        <>
          <label className={styles.field}>
            <span>Side</span>
            <Select
              value={form.side ?? ""}
              onChange={(event) =>
                set({
                  side:
                    event.target.value === "long" ||
                    event.target.value === "short" ||
                    event.target.value === "net"
                      ? event.target.value
                      : undefined,
                })
              }
            >
              <option value="long">long</option>
              <option value="short">short</option>
              <option value="net">net</option>
            </Select>
          </label>
          <Field
            label="Quantity"
            value={form.quantity ?? ""}
            onChange={(value) => set({ quantity: value || undefined })}
          />
          <Field
            label="Entry price"
            value={form.entryPrice ?? ""}
            onChange={(value) => set({ entryPrice: emptyToNull(value) })}
          />
          <Field
            label="Mark price"
            value={form.markPrice ?? ""}
            onChange={(value) => set({ markPrice: emptyToNull(value) })}
          />
          <Field
            label="Unrealized value"
            value={form.unrealizedValue ?? ""}
            onChange={(value) => set({ unrealizedValue: emptyToNull(value) })}
          />
          <Field
            label="Leverage"
            value={form.leverage ?? ""}
            onChange={(value) => set({ leverage: emptyToNull(value) })}
          />
        </>
      ) : null}
      {kind === "reported_equity" ? (
        <Field
          label="Amount"
          value={form.amount ?? ""}
          onChange={(value) => set({ amount: value || undefined })}
        />
      ) : null}
      {kind === "price" ? (
        <Field
          label="Price"
          value={form.price ?? ""}
          onChange={(value) => set({ price: value || undefined })}
        />
      ) : null}
      {kind === "account_metric" ? (
        <>
          <Field
            label="Decimal value"
            value={form.decimalValue ?? ""}
            onChange={(value) => set({ decimalValue: emptyToNull(value) })}
          />
          <Field
            label="Text value"
            value={form.textValue ?? ""}
            onChange={(value) => set({ textValue: emptyToNull(value) })}
          />
        </>
      ) : null}
    </>
  );
}

function Field({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className={styles.field}>
      <span>{label}</span>
      <TextInput value={value} onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

function formFromObservation(item: Observation): PatchObservationRequest {
  return {
    completeness: item.completeness,
    totalQuantity: item.balance?.totalQuantity ?? undefined,
    availableQuantity: item.balance?.availableQuantity ?? undefined,
    lockedQuantity: item.balance?.lockedQuantity ?? undefined,
    side: item.position?.side,
    quantity: item.position?.quantity,
    entryPrice: item.position?.entryPrice ?? undefined,
    markPrice: item.position?.markPrice ?? undefined,
    unrealizedValue: item.position?.unrealizedValue ?? undefined,
    leverage: item.position?.leverage ?? undefined,
    amount: item.reportedEquity?.amount,
    price: item.price?.price,
    decimalValue: item.accountMetric?.decimalValue ?? undefined,
    textValue: item.accountMetric?.textValue ?? undefined,
  };
}

function typedPayload(item: Observation): unknown {
  switch (item.kind) {
    case "balance":
      return item.balance;
    case "position":
      return item.position;
    case "reported_equity":
      return item.reportedEquity;
    case "price":
      return item.price;
    case "account_metric":
      return item.accountMetric;
    default:
      return null;
  }
}

function emptyToNull(value: string): string | null {
  return value.trim() ? value : null;
}

function readKind(value: string | null): ObservationKind | "" {
  return KINDS.includes(value as (typeof KINDS)[number]) ? (value as ObservationKind | "") : "";
}
