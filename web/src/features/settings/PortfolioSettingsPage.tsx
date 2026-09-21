import { type FormEvent, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import {
  deletePortfolio,
  getPortfolio,
  listPortfolios,
  patchPortfolio,
} from "@/api/generated/sdk.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY } from "@/app/shell/navigation";
import { storePreference } from "@/shared/lib/preferences";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

const DISPLAY_ASSETS = [
  { id: "01a0bdfc-1fa4-7a54-9fa1-24f4721a71af", code: "USD" },
  { id: "01a0bdfc-1fa4-7af4-bedf-65d968b9122b", code: "CNY" },
] as const;

export function PortfolioSettingsPage() {
  const navigate = useNavigate();
  const { portfolioId = "" } = useParams();
  const [name, setName] = useState("");
  const [timezone, setTimezone] = useState("Asia/Shanghai");
  const [displayAssetId, setDisplayAssetId] = useState<string>(DISPLAY_ASSETS[0].id);
  const [etag, setEtag] = useState("");
  const [undeletable, setUndeletable] = useState(false);
  const [confirmName, setConfirmName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const result = await getPortfolio({ path: { portfolioId } });
        if (result.response?.status === 401) {
          void navigate("/login", { replace: true });
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The portfolio could not be loaded."));
          return;
        }
        setName(result.data.name);
        setTimezone(result.data.timezone);
        setDisplayAssetId(result.data.displayAsset.id);
        setUndeletable(result.data.undeletable);
        setConfirmName("");
        setEtag(result.response?.headers.get("ETag") ?? `"portfolio:${result.data.version}"`);
      } catch (cause) {
        setError(
          problemMessage(cause, "The portfolio could not be loaded. Check the network and retry."),
        );
      }
    })();
  }, [navigate, portfolioId]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const result = await patchPortfolio({
        path: { portfolioId },
        headers: { "If-Match": etag },
        body: { name, timezone, displayAssetId },
      });
      if (result.response?.status === 401) {
        void navigate("/login", { replace: true });
        return;
      }
      if (result.response?.status === 412) {
        setError("The portfolio changed in another session. Reload before saving.");
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The portfolio could not be updated."));
        return;
      }
      setName(result.data.name);
      setTimezone(result.data.timezone);
      setDisplayAssetId(result.data.displayAsset.id);
      setUndeletable(result.data.undeletable);
      setEtag(result.response?.headers.get("ETag") ?? `"portfolio:${result.data.version}"`);
      setNotice("Portfolio settings saved.");
    } catch (cause) {
      setError(
        problemMessage(cause, "The portfolio could not be updated. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(event: FormEvent) {
    event.preventDefault();
    if (confirmName !== name) {
      setError("Type the current portfolio name to delete it.");
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const result = await deletePortfolio({
        path: { portfolioId },
        headers: { "If-Match": etag },
      });
      if (result.response?.status === 401) {
        void navigate("/login", { replace: true });
        return;
      }
      if (result.response?.status === 412) {
        setError("The portfolio changed in another session. Reload before deleting.");
        return;
      }
      if (result.error || result.response?.status !== 204) {
        setError(problemMessage(result.error, "The portfolio could not be deleted."));
        return;
      }
      const listed = await listPortfolios();
      const next = listed.data?.items.find((item) => item.undeletable) ?? listed.data?.items[0];
      if (!next) {
        void navigate("/", { replace: true });
        return;
      }
      storePreference(LAST_PORTFOLIO_KEY, next.id);
      void navigate(`/p/${next.id}/overview`, { replace: true });
    } catch (cause) {
      setError(
        problemMessage(cause, "The portfolio could not be deleted. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.page}>
      <h2>Portfolio settings</h2>
      <p className={styles.lead}>
        Rename this workspace, choose the timezone used for period boundaries, and pick the display
        Asset.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
        <label className={styles.field}>
          <span>Name</span>
          <TextInput value={name} onChange={(event) => setName(event.target.value)} required />
        </label>
        <label className={styles.field}>
          <span>Timezone</span>
          <TextInput
            value={timezone}
            onChange={(event) => setTimezone(event.target.value)}
            required
          />
        </label>
        <label className={styles.field}>
          <span>Display asset</span>
          <Select
            value={displayAssetId}
            onChange={(event) => setDisplayAssetId(event.target.value)}
            required
          >
            {DISPLAY_ASSETS.map((asset) => (
              <option key={asset.id} value={asset.id}>
                {asset.code}
              </option>
            ))}
          </Select>
        </label>
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Save
          </ActionButton>
        </div>
      </form>
      {undeletable ? (
        <p className={styles.notice}>
          This portfolio was created with the account and cannot be deleted.
        </p>
      ) : (
        <form className={styles.form} onSubmit={(event) => void onDelete(event)}>
          <h3>Delete portfolio</h3>
          <p className={styles.lead}>
            This permanently deletes the workspace and every Identity, Observation, Capital row,
            job, and projection scoped to it. Type the current name to confirm.
          </p>
          <label className={styles.field}>
            <span>Confirm name</span>
            <TextInput
              value={confirmName}
              onChange={(event) => setConfirmName(event.target.value)}
              autoComplete="off"
              required
            />
          </label>
          <div className={styles.actions}>
            <ActionButton type="submit" tone="danger" disabled={busy || confirmName !== name}>
              Delete
            </ActionButton>
          </div>
        </form>
      )}
    </section>
  );
}
