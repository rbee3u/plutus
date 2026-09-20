import { type FormEvent, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getCapitalBaseline,
  putCapitalBaseline,
  searchCatalogAssets,
} from "@/api/generated/sdk.gen";
import type { CapitalBaseline, CatalogAsset } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { fromDatetimeLocalValue, toDatetimeLocalValue } from "@/shared/lib/datetime";
import {
  readEtag,
  SessionExpiredError,
  unwrapOrRedirect,
  useCurrentUser,
} from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Select, TextArea, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

interface BaselineView {
  item: CapitalBaseline | null;
  etag: string;
}

export function BaselinePage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const redirect = () => {
    void navigate("/login", { replace: true });
  };

  const assets = useQuery({
    queryKey: ["catalog-assets", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await searchCatalogAssets({ path: { portfolioId }, query: { limit: 200 } }),
        redirect,
      ),
  });

  const baseline = useQuery({
    queryKey: ["capital-baseline", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async (): Promise<BaselineView> => {
      const result = await getCapitalBaseline({ path: { portfolioId } });
      if (result.response?.status === 401) {
        redirect();
        throw new SessionExpiredError();
      }
      if (result.response?.status === 404) {
        return { item: null, etag: "*" };
      }
      const item = unwrapOrRedirect(result, redirect);
      return {
        item,
        etag: readEtag(result.response, `"capital-baseline:${String(item.version)}"`),
      };
    },
  });

  return (
    <section className={styles.page}>
      <h2>Capital baseline</h2>
      <p className={styles.lead}>
        One logical baseline per Portfolio. Saving appends a revision and keeps the original Asset
        quantity.
      </p>
      <QueryRecovery
        error={baseline.error}
        hasData={Boolean(baseline.data)}
        busy={baseline.isFetching}
        fallback="The baseline could not be loaded."
        onRetry={() => {
          void baseline.refetch();
        }}
      />
      {baseline.data ? (
        <BaselineForm
          key={`${baseline.data.etag}-${baseline.data.item?.id ?? "new"}`}
          portfolioId={portfolioId}
          view={baseline.data}
          assets={assets.data?.items ?? []}
          redirect={redirect}
          onInvalidate={async () => {
            await queryClient.invalidateQueries({
              queryKey: ["overview", user.data?.id, portfolioId],
            });
            await queryClient.invalidateQueries({
              queryKey: ["capital-baseline", user.data?.id, portfolioId],
            });
          }}
        />
      ) : (
        <p className={styles.muted}>Loading capital baseline.</p>
      )}
    </section>
  );
}

function BaselineForm({
  portfolioId,
  view,
  assets,
  redirect,
  onInvalidate,
}: {
  portfolioId: string;
  view: BaselineView;
  assets: CatalogAsset[];
  redirect: () => void;
  onInvalidate: () => Promise<void>;
}) {
  const idempotencyKey = useRef(crypto.randomUUID());
  const [effectiveAt, setEffectiveAt] = useState(
    view.item ? toDatetimeLocalValue(view.item.effectiveAt) : "",
  );
  const [assetId, setAssetId] = useState(view.item?.assetId ?? "");
  const [quantity, setQuantity] = useState(view.item?.quantity ?? "");
  const [note, setNote] = useState(view.item?.note ?? "");
  const [etag, setEtag] = useState(view.etag);
  const [exists, setExists] = useState(view.item !== null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function applyItem(item: CapitalBaseline, nextEtag: string) {
    setExists(true);
    setEffectiveAt(toDatetimeLocalValue(item.effectiveAt));
    setAssetId(item.assetId);
    setQuantity(item.quantity);
    setNote(item.note ?? "");
    setEtag(nextEtag);
  }

  async function loadLatest(message: string) {
    const result = await getCapitalBaseline({ path: { portfolioId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      applyItem(
        result.data,
        readEtag(result.response, `"capital-baseline:${String(result.data.version)}"`),
      );
    }
    setError(message);
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const result = await putCapitalBaseline({
        path: { portfolioId },
        headers: {
          "If-Match": etag,
          "Idempotency-Key": idempotencyKey.current,
        },
        body: {
          effectiveAt: fromDatetimeLocalValue(effectiveAt),
          assetId,
          quantity,
          note: note.trim() ? note : null,
        },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await loadLatest(
          "The baseline changed in another session. The form now shows the latest server values.",
        );
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The capital baseline could not be saved."));
        return;
      }
      applyItem(
        result.data.baseline,
        readEtag(result.response, `"capital-baseline:${String(result.data.baseline.version)}"`),
      );
      idempotencyKey.current = crypto.randomUUID();
      setNotice("Capital baseline saved. A projection rebuild was queued.");
      await onInvalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The capital baseline could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
        <label className={styles.field}>
          <span>Effective at</span>
          <TextInput
            type="datetime-local"
            value={effectiveAt}
            onChange={(event) => setEffectiveAt(event.target.value)}
            required
          />
        </label>
        <label className={styles.field}>
          <span>Asset</span>
          <Select value={assetId} onChange={(event) => setAssetId(event.target.value)} required>
            <option value="" disabled>
              Select an asset
            </option>
            {assets.map((item) => (
              <option key={item.id} value={item.id}>
                {item.code}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Quantity</span>
          <TextInput
            value={quantity}
            onChange={(event) => setQuantity(event.target.value)}
            required
          />
        </label>
        <label className={styles.field}>
          <span>Note</span>
          <TextArea value={note} onChange={(event) => setNote(event.target.value)} rows={3} />
        </label>
        <p className={styles.muted}>
          {exists
            ? "Editing saves a new revision and moves the current pointer."
            : "No baseline exists yet. Saving creates the first revision."}
        </p>
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Save baseline
          </ActionButton>
        </div>
      </form>
    </>
  );
}
