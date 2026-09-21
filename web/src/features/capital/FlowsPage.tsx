import { type FormEvent, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createCapitalFlow,
  deleteCapitalFlow,
  getCapitalFlow,
  listAccounts,
  listCapitalFlows,
  patchCapitalFlow,
  searchCatalogAssets,
} from "@/api/generated/sdk.gen";
import type { CapitalFlow } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant, fromDatetimeLocalValue, toDatetimeLocalValue } from "@/shared/lib/datetime";
import { newIdempotencyKey } from "@/shared/lib/id";
import { readEtag, unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Pager } from "@/shared/ui/Pager";
import { Select, TextArea, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

interface FlowForm {
  direction: "contribution" | "withdrawal";
  quantity: string;
  assetId: string;
  effectiveAt: string;
  accountId: string;
  note: string;
}

const emptyForm: FlowForm = {
  direction: "contribution",
  quantity: "",
  assetId: "",
  effectiveAt: "",
  accountId: "",
  note: "",
};

export function FlowsPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const [params, setParams] = useSearchParams();
  const offset = Math.max(0, Number(params.get("offset") ?? "0") || 0);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const createKey = useRef(newIdempotencyKey());
  const [form, setForm] = useState<FlowForm>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [etag, setEtag] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const assets = useQuery({
    queryKey: ["catalog-assets", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await searchCatalogAssets({ path: { portfolioId }, query: { limit: 200 } }),
        redirect,
      ),
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
  const flows = useQuery({
    queryKey: ["capital-flows", user.data?.id, portfolioId, offset],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listCapitalFlows({ path: { portfolioId }, query: { offset, limit: PAGE_SIZE } }),
        redirect,
      ),
  });

  const assetLabel = useMemo(() => {
    const map = new Map((assets.data?.items ?? []).map((item) => [item.id, item.code]));
    return (id: string) => map.get(id) ?? id;
  }, [assets.data]);

  function applyFlow(item: CapitalFlow, nextEtag: string) {
    setEditingId(item.id);
    setEtag(nextEtag);
    setForm({
      direction: item.direction,
      quantity: item.quantity,
      assetId: item.assetId,
      effectiveAt: toDatetimeLocalValue(item.effectiveAt),
      accountId: item.accountId ?? "",
      note: item.note ?? "",
    });
  }

  async function startEdit(flowId: string) {
    setError(null);
    setNotice(null);
    const result = await getCapitalFlow({ path: { portfolioId, flowId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    const item = unwrapOrRedirect(result, redirect);
    applyFlow(item, readEtag(result.response, `"capital-flow:${String(item.version)}"`));
  }

  async function loadLatest(flowId: string, message: string) {
    const result = await getCapitalFlow({ path: { portfolioId, flowId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      applyFlow(
        result.data,
        readEtag(result.response, `"capital-flow:${String(result.data.version)}"`),
      );
    }
    setError(message);
  }

  async function invalidate() {
    await queryClient.invalidateQueries({
      queryKey: ["capital-flows", user.data?.id, portfolioId],
    });
    await queryClient.invalidateQueries({ queryKey: ["overview", user.data?.id, portfolioId] });
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const body = {
        direction: form.direction,
        quantity: form.quantity,
        assetId: form.assetId,
        effectiveAt: fromDatetimeLocalValue(form.effectiveAt),
        accountId: form.accountId || null,
        note: form.note.trim() ? form.note : null,
      };
      if (editingId) {
        const result = await patchCapitalFlow({
          path: { portfolioId, flowId: editingId },
          headers: { "If-Match": etag },
          body,
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.response?.status === 412) {
          await loadLatest(
            editingId,
            "The flow changed in another session. The form now shows the latest server values.",
          );
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The capital flow could not be updated."));
          return;
        }
        applyFlow(
          result.data.flow,
          readEtag(result.response, `"capital-flow:${String(result.data.flow.version)}"`),
        );
        setNotice("Capital flow updated. A projection rebuild was queued.");
      } else {
        const result = await createCapitalFlow({
          path: { portfolioId },
          headers: { "Idempotency-Key": createKey.current },
          body,
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The capital flow could not be created."));
          return;
        }
        createKey.current = newIdempotencyKey();
        setForm(emptyForm);
        setNotice("Capital flow created. A projection rebuild was queued.");
      }
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The capital flow could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(flowId: string) {
    if (!window.confirm("Delete this capital flow? History is kept as a deleted revision.")) {
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const current = await getCapitalFlow({ path: { portfolioId, flowId } });
      if (current.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(current, redirect);
      const match = readEtag(current.response, `"capital-flow:${String(item.version)}"`);
      await refreshCsrf();
      const result = await deleteCapitalFlow({
        path: { portfolioId, flowId },
        headers: { "If-Match": match },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await loadLatest(
          flowId,
          "The flow changed in another session. The form now shows the latest server values.",
        );
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The capital flow could not be deleted."));
        return;
      }
      if (editingId === flowId) {
        setEditingId(null);
        setForm(emptyForm);
        setEtag("");
      }
      setNotice("Capital flow deleted. A projection rebuild was queued.");
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The capital flow could not be deleted."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.wide}>
      <h2>Capital flows</h2>
      <p className={styles.lead}>
        Contributions and withdrawals keep their original Asset quantity. Editing appends a
        revision.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <QueryRecovery
        error={flows.error}
        hasData={Boolean(flows.data)}
        busy={flows.isFetching}
        fallback="Capital flows could not be loaded."
        onRetry={() => {
          void flows.refetch();
        }}
      />
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
        <label className={styles.field}>
          <span>Direction</span>
          <Select
            value={form.direction}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                direction: event.target.value as FlowForm["direction"],
              }))
            }
            required
          >
            <option value="contribution">Contribution</option>
            <option value="withdrawal">Withdrawal</option>
          </Select>
        </label>
        <label className={styles.field}>
          <span>Effective at</span>
          <TextInput
            type="datetime-local"
            value={form.effectiveAt}
            onChange={(event) =>
              setForm((current) => ({ ...current, effectiveAt: event.target.value }))
            }
            required
          />
        </label>
        <label className={styles.field}>
          <span>Asset</span>
          <Select
            value={form.assetId}
            onChange={(event) =>
              setForm((current) => ({ ...current, assetId: event.target.value }))
            }
            required
          >
            <option value="" disabled>
              Select an asset
            </option>
            {(assets.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.code}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Quantity</span>
          <TextInput
            value={form.quantity}
            onChange={(event) =>
              setForm((current) => ({ ...current, quantity: event.target.value }))
            }
            required
          />
        </label>
        <label className={styles.field}>
          <span>Account</span>
          <Select
            value={form.accountId}
            onChange={(event) =>
              setForm((current) => ({ ...current, accountId: event.target.value }))
            }
          >
            <option value="">None</option>
            {(accounts.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.displayName}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Note</span>
          <TextArea
            value={form.note}
            onChange={(event) => setForm((current) => ({ ...current, note: event.target.value }))}
            rows={3}
          />
        </label>
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            {editingId ? "Save revision" : "Create flow"}
          </ActionButton>
          {editingId ? (
            <ActionButton
              type="button"
              onClick={() => {
                setEditingId(null);
                setForm(emptyForm);
                setEtag("");
              }}
            >
              Cancel edit
            </ActionButton>
          ) : null}
        </div>
      </form>
      <DataTable
        caption="Active capital flows"
        columns={[
          { key: "direction", header: "Direction" },
          { key: "quantity", header: "Quantity", numeric: true },
          { key: "asset", header: "Asset" },
          { key: "effectiveAt", header: "Effective at" },
          { key: "note", header: "Note" },
        ]}
        rows={(flows.data?.items ?? []).map((item) => ({
          id: item.id,
          direction: item.direction,
          quantity: formatDecimal(item.quantity),
          asset: assetLabel(item.assetId),
          effectiveAt: formatInstant(item.effectiveAt),
          note: item.note ?? "—",
        }))}
      />
      <FlowActions
        items={flows.data?.items ?? []}
        busy={busy}
        onEdit={(id) => void startEdit(id)}
        onDelete={(id) => void onDelete(id)}
      />
      <Pager
        offset={offset}
        limit={PAGE_SIZE}
        total={flows.data?.page.total ?? 0}
        onChange={(nextOffset) => {
          const merged = new URLSearchParams(params);
          if (nextOffset > 0) merged.set("offset", String(nextOffset));
          else merged.delete("offset");
          setParams(merged, { replace: true });
        }}
      />
    </section>
  );
}

function FlowActions({
  items,
  busy,
  onEdit,
  onDelete,
}: {
  items: CapitalFlow[];
  busy: boolean;
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  if (items.length === 0) {
    return null;
  }
  return (
    <ul className={styles.flowActions}>
      {items.map((item) => (
        <li key={item.id}>
          <span>
            {item.direction} {formatDecimal(item.quantity)}
          </span>
          <span className={styles.actions}>
            <ActionButton type="button" disabled={busy} onClick={() => onEdit(item.id)}>
              Edit
            </ActionButton>
            <ActionButton
              type="button"
              tone="dangerQuiet"
              disabled={busy}
              onClick={() => onDelete(item.id)}
            >
              Delete
            </ActionButton>
          </span>
        </li>
      ))}
    </ul>
  );
}
