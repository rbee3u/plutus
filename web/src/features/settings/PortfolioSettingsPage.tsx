import { formatCount } from "@/shared/lib/amount";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { deletePortfolio, listPortfolios, patchPortfolio } from "@/api/generated/sdk.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY } from "@/app/shell/navigation";
import { type PortfolioView, usePortfolio } from "@/shared/lib/portfolio";
import { storePreference } from "@/shared/lib/preferences";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const MIN_INTERVAL = 60;
const MAX_INTERVAL = 86_400;
const CONFLICT =
  "The portfolio changed in another session. The latest values were reloaded; review and save again.";

export function PortfolioSettingsPage() {
  const { portfolioId = "" } = useParams();
  const { userId } = useSessionScope();
  const queryClient = useQueryClient();
  const portfolio = usePortfolio(portfolioId);
  const [notice, setNotice] = useState<string | null>(null);
  const [conflict, setConflict] = useState<string | null>(null);

  async function reload() {
    await queryClient.invalidateQueries({ queryKey: ["portfolio", userId, portfolioId] });
  }

  return (
    <section className={styles.page}>
      <p className={styles.lead}>
        The name, the timezone that closes days, weeks, months, quarters, and years, and how often
        Collection Cycles run. Valuation is always in USD.
      </p>
      {conflict ? (
        <p className={styles.alert} role="alert">
          {conflict}
        </p>
      ) : null}
      {notice ? (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      ) : null}
      <QueryRecovery
        error={portfolio.error}
        hasData={Boolean(portfolio.data)}
        busy={portfolio.isFetching}
        fallback="The portfolio could not be loaded."
        onRetry={() => {
          void portfolio.refetch();
        }}
      />
      {portfolio.data ? (
        <SettingsForms
          key={portfolio.data.etag}
          portfolioId={portfolioId}
          view={portfolio.data}
          onSaved={async (message) => {
            setConflict(null);
            setNotice(message);
            await reload();
          }}
          onConflict={async () => {
            setNotice(null);
            setConflict(CONFLICT);
            await reload();
          }}
        />
      ) : portfolio.isPending ? (
        <p className={styles.muted}>Loading portfolio settings.</p>
      ) : null}
    </section>
  );
}

function SettingsForms({
  portfolioId,
  view,
  onSaved,
  onConflict,
}: {
  portfolioId: string;
  view: PortfolioView;
  onSaved: (message: string) => Promise<void>;
  onConflict: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const { redirect } = useSessionScope();
  const [name, setName] = useState(view.item.name);
  const [timezone, setTimezone] = useState(view.item.timezone);
  const [interval, setInterval] = useState(String(view.item.collectionIntervalSeconds));
  const [confirmName, setConfirmName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    const seconds = Number(interval);
    if (!Number.isInteger(seconds) || seconds < MIN_INTERVAL || seconds > MAX_INTERVAL) {
      setError(
        `The collection interval must be a whole number of seconds between ${formatCount(MIN_INTERVAL)} and ${formatCount(MAX_INTERVAL)}.`,
      );
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await patchPortfolio({
        path: { portfolioId },
        headers: { "If-Match": view.etag },
        body: {
          name: name.trim(),
          timezone: timezone.trim(),
          collectionIntervalSeconds: seconds,
        },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onConflict();
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The portfolio could not be updated."));
        return;
      }
      await onSaved("Portfolio settings saved.");
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
    if (confirmName !== view.item.name) {
      setError("Type the current portfolio name to delete it.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await deletePortfolio({
        path: { portfolioId },
        headers: { "If-Match": view.etag },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        await onConflict();
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
    <>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
        <label className={styles.field}>
          <span>Name</span>
          <TextInput
            value={name}
            onChange={(event) => setName(event.target.value)}
            maxLength={120}
            required
          />
        </label>
        <label className={styles.field}>
          <span>Timezone</span>
          <TextInput
            value={timezone}
            onChange={(event) => setTimezone(event.target.value)}
            placeholder="IANA name such as Asia/Shanghai"
            required
          />
        </label>
        <label className={styles.field}>
          <span>Collection interval (seconds)</span>
          <TextInput
            type="number"
            inputMode="numeric"
            min={MIN_INTERVAL}
            max={MAX_INTERVAL}
            step={1}
            value={interval}
            onChange={(event) => setInterval(event.target.value)}
            required
          />
          <span className={styles.muted}>
            Between {formatCount(MIN_INTERVAL)} seconds and {formatCount(MAX_INTERVAL)} seconds (one
            day). The schedule applies from the next cycle.
          </span>
        </label>
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Save
          </ActionButton>
        </div>
      </form>
      {view.item.undeletable ? (
        <p className={styles.notice}>
          This portfolio was created with the account and cannot be deleted.
        </p>
      ) : (
        <form className={styles.form} onSubmit={(event) => void onDelete(event)}>
          <h2>Delete portfolio</h2>
          <p className={styles.lead}>
            This permanently deletes the workspace and every Identity, Collection Cycle, Valuation
            Point, Capital Flow, and job scoped to it. Type the current name to confirm.
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
            <ActionButton
              type="submit"
              tone="danger"
              disabled={busy || confirmName !== view.item.name}
            >
              Delete
            </ActionButton>
          </div>
        </form>
      )}
    </>
  );
}
