import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { portfoliosOptions } from "@/shared/lib/usePortfolios";
import { Shield } from "lucide-react";
import { formatCount } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { useQueryClient } from "@tanstack/react-query";
import { type ChangeEvent, type FormEvent, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { deletePortfolio, keys, patchPortfolio, problemMessage, type Portfolio } from "@/api";
import { LAST_PORTFOLIO_KEY } from "@/shared/lib/preferences";
import { portfolioEtag, usePortfolio } from "@/shared/lib/portfolio";
import { storePreference } from "@/shared/lib/preferences";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/settings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

const MIN_INTERVAL = 900;
const MAX_INTERVAL = 86_400;
const DEFAULT_INTERVAL = 900;
const CONFLICT =
  "The portfolio changed in another session. Reload the latest settings to review before saving again.";

const TIMEZONE_OPTIONS = [
  { value: "UTC", label: "UTC (UTC+0 · Coordinated Universal Time)" },
  { value: "Asia/Shanghai", label: "Asia/Shanghai (UTC+8 · Beijing, Shanghai, HK, SG)" },
  { value: "Asia/Tokyo", label: "Asia/Tokyo (UTC+9 · Tokyo, Seoul)" },
  { value: "America/New_York", label: "America/New_York (UTC-5/-4 · US Eastern, New York)" },
  { value: "America/Los_Angeles", label: "America/Los_Angeles (UTC-8/-7 · US Pacific, LA)" },
  { value: "Europe/London", label: "Europe/London (UTC+0/+1 · London)" },
  { value: "Europe/Berlin", label: "Europe/Berlin (UTC+1/+2 · Berlin, Frankfurt)" },
] as const;

const INTERVAL_PRESETS = [
  { label: "15m", seconds: 900, isDefault: true },
  { label: "30m", seconds: 1800 },
  { label: "1h", seconds: 3600 },
  { label: "2h", seconds: 7200 },
  { label: "6h", seconds: 21600 },
  { label: "12h", seconds: 43200 },
  { label: "24h", seconds: 86400 },
] as const;

function formatIntervalDuration(seconds: number): string | null {
  if (!Number.isFinite(seconds) || seconds < MIN_INTERVAL || seconds > MAX_INTERVAL) {
    return null;
  }
  if (seconds % 86400 === 0) {
    const days = seconds / 86400;
    return `${days} ${days === 1 ? "day" : "days"}`;
  }
  if (seconds % 3600 === 0) {
    const hours = seconds / 3600;
    return `${hours} ${hours === 1 ? "hour" : "hours"}`;
  }
  if (seconds % 60 === 0) {
    const minutes = seconds / 60;
    if (minutes < 60) return `${minutes} ${minutes === 1 ? "minute" : "minutes"}`;
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;
    return `${h}h ${m}m`;
  }
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  return `${m}m ${s}s`;
}

export function PortfolioSettingsPage() {
  const { portfolioId = "" } = useParams();
  const queryClient = useQueryClient();
  const portfolio = usePortfolio(portfolioId);
  const { userId } = useSessionScope();

  async function reload() {
    await queryClient.invalidateQueries({ queryKey: keys.portfolio(userId, portfolioId) });
    await queryClient.invalidateQueries({ queryKey: keys.portfolios(userId) });
  }

  return (
    <section className={styles.page}>
      <div className={shared.panelHeader} style={{ flexWrap: "wrap", gap: "var(--space-md)" }}>
        <div>
          <h2 style={{ margin: 0 }}>Portfolio settings</h2>
          <p className={shared.muted} style={{ margin: "4px 0 0", fontSize: "var(--text-sm)" }}>
            Manage configuration, base timezone, and collection cadence for this portfolio.
          </p>
        </div>
      </div>
      <QueryRecovery query={portfolio} fallback="The portfolio could not be loaded." />
      {portfolio.data ? (
        <SettingsForms
          key={portfolioId}
          portfolioId={portfolioId}
          initialPortfolio={portfolio.data}
          onSaved={reload}
          onReload={async () => {
            const result = await portfolio.refetch();
            if (result.error) throw result.error;
            if (!result.data) throw new Error("The portfolio could not be loaded.");
            return result.data;
          }}
        />
      ) : portfolio.isPending ? (
        <p className={shared.muted}>Loading portfolio settings.</p>
      ) : null}
    </section>
  );
}

function SettingsForms({
  portfolioId,
  initialPortfolio,
  onSaved,
  onReload,
}: {
  portfolioId: string;
  initialPortfolio: Portfolio;
  onSaved: () => Promise<void>;
  onReload: () => Promise<Portfolio>;
}) {
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const write = useAuthenticatedWrite();
  const navigate = useNavigate();
  const [notice, setNotice] = useState<string | null>(null);
  const [baseline, setBaseline] = useState(initialPortfolio);
  const [needsReload, setNeedsReload] = useState(false);
  const [name, setName] = useState(initialPortfolio.name);
  const [timezone, setTimezone] = useState(initialPortfolio.timezone);
  const [interval, setInterval] = useState(String(initialPortfolio.collectionIntervalSeconds));
  const [confirmName, setConfirmName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  let browserTimezone: string | null = null;
  try {
    browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || null;
  } catch {
    browserTimezone = null;
  }
  const canMatchBrowser = Boolean(browserTimezone && browserTimezone !== timezone.trim());
  const isKnownTimezone = TIMEZONE_OPTIONS.some((opt) => opt.value === timezone);

  const numSeconds = interval === "" ? NaN : Number(interval);
  const durationText = formatIntervalDuration(numSeconds);
  const isIntervalBelowMin = !Number.isNaN(numSeconds) && numSeconds < MIN_INTERVAL;
  const isIntervalEmpty = interval === "";

  function onIntervalChange(event: ChangeEvent<HTMLInputElement>) {
    let clean = event.target.value.replace(/\D/g, "");
    clean = clean.replace(/^0+/, "");
    if (clean !== "") {
      const num = Number(clean);
      if (num > MAX_INTERVAL) {
        clean = String(MAX_INTERVAL);
      }
    }
    setInterval(clean);
  }

  function onIntervalBlur() {
    if (interval === "") {
      setInterval(String(baseline.collectionIntervalSeconds || DEFAULT_INTERVAL));
      return;
    }
    const num = Number(interval);
    if (num < MIN_INTERVAL) {
      setInterval(String(MIN_INTERVAL));
    } else if (num > MAX_INTERVAL) {
      setInterval(String(MAX_INTERVAL));
    }
  }

  function acceptPortfolio(item: Portfolio) {
    setBaseline(item);
    setName(item.name);
    setTimezone(item.timezone);
    setInterval(String(item.collectionIntervalSeconds));
    setConfirmName("");
    setNeedsReload(false);
  }

  async function reloadLatest() {
    setNotice(null);
    setBusy(true);
    try {
      acceptPortfolio(await onReload());
      setError(null);
    } catch (cause) {
      setError(problemMessage(cause, "The portfolio could not be loaded."));
    } finally {
      setBusy(false);
    }
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (busy || needsReload) return;
    let seconds = Number(interval);
    if (interval === "" || seconds < MIN_INTERVAL) {
      seconds = MIN_INTERVAL;
      setInterval(String(MIN_INTERVAL));
    } else if (seconds > MAX_INTERVAL) {
      seconds = MAX_INTERVAL;
      setInterval(String(MAX_INTERVAL));
    }
    if (!Number.isInteger(seconds) || seconds < MIN_INTERVAL || seconds > MAX_INTERVAL) {
      setError(
        `The collection interval must be a whole number of seconds between ${formatCount(MIN_INTERVAL)} and ${formatCount(MAX_INTERVAL)}.`,
      );
      return;
    }
    setNotice(null);
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        patchPortfolio({
          path: { portfolioId },
          headers: { "If-Match": portfolioEtag(baseline) },
          body: {
            name: name.trim(),
            timezone: timezone.trim(),
            collectionIntervalSeconds: seconds,
          },
        }),
      );
      if (result.response?.status === 412) {
        setNeedsReload(true);
        setError(CONFLICT);
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The portfolio could not be updated."));
        return;
      }
      acceptPortfolio(result.data);
      setNotice("Portfolio settings saved.");
      await onSaved();
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
    if (busy || needsReload) return;
    if (confirmName !== baseline.name) {
      setError("Type the current portfolio name to delete it.");
      return;
    }
    setNotice(null);
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        deletePortfolio({
          path: { portfolioId },
          headers: { "If-Match": portfolioEtag(baseline) },
        }),
      );
      if (result.response?.status === 412) {
        setNeedsReload(true);
        setError(CONFLICT);
        return;
      }
      if (result.error || result.response?.status !== 202) {
        setError(problemMessage(result.error, "The portfolio could not be deleted."));
        return;
      }
      await queryClient.invalidateQueries({ queryKey: keys.portfolios(userId) });
      const listed = await queryClient.fetchQuery(portfoliosOptions(userId, redirect));
      const next = listed.items.find((item) => item.undeletable) ?? listed.items[0];
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
      {notice ? (
        <p className={shared.notice} role="status">
          {notice}
        </p>
      ) : null}
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      {needsReload ? (
        <ActionButton type="button" disabled={busy} onClick={() => void reloadLatest()}>
          Reload latest settings
        </ActionButton>
      ) : null}
      <form
        className={`${shared.form} ${styles.settingsCard}`}
        onSubmit={(event) => void onSubmit(event)}
      >
        <h2>General settings</h2>
        <label className={shared.field}>
          <span>Name</span>
          <TextInput
            value={name}
            disabled={busy}
            onChange={(event) => setName(event.target.value)}
            maxLength={120}
            required
          />
        </label>
        <label className={shared.field}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
            <span>Timezone</span>
            {canMatchBrowser && browserTimezone ? (
              <button
                type="button"
                className={styles.tzSuggestionBtn}
                disabled={busy}
                onClick={() => setTimezone(browserTimezone)}
                title={`Set timezone to ${browserTimezone}`}
              >
                Match browser ({browserTimezone})
              </button>
            ) : null}
          </div>
          <Select
            disabled={busy}
            value={timezone}
            onChange={(event) => setTimezone(event.target.value)}
            required
          >
            {!isKnownTimezone && timezone ? <option value={timezone}>{timezone}</option> : null}
            {TIMEZONE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </Select>
        </label>
        <label className={shared.field}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
            <span>Collection interval (seconds)</span>
            {durationText ? (
              <span className={styles.intervalDurationBadge}>{durationText}</span>
            ) : isIntervalBelowMin || isIntervalEmpty ? (
              <span className={styles.intervalDurationBadgeWarn}>
                Min {formatCount(MIN_INTERVAL)}s (15m)
              </span>
            ) : null}
          </div>
          <TextInput
            type="text"
            inputMode="numeric"
            pattern="[0-9]*"
            value={interval}
            disabled={busy}
            onChange={onIntervalChange}
            onBlur={onIntervalBlur}
            required
          />
          <div className={styles.intervalPresets} role="group" aria-label="Interval presets">
            <span className={styles.intervalPresetsLabel}>Presets:</span>
            {INTERVAL_PRESETS.map((p) => {
              const active = interval === String(p.seconds);
              const presetTitle =
                "isDefault" in p && p.isDefault
                  ? `${p.label} (default, ${formatCount(p.seconds)}s)`
                  : `${p.label} (${formatCount(p.seconds)}s)`;
              return (
                <button
                  key={p.seconds}
                  disabled={busy}
                  type="button"
                  aria-pressed={active}
                  className={`${styles.intervalPresetBtn} ${active ? styles.intervalPresetActive : ""}`}
                  onClick={() => setInterval(String(p.seconds))}
                  title={presetTitle}
                >
                  {p.label}
                </button>
              );
            })}
          </div>
          <span className={shared.muted}>
            Between {formatCount(MIN_INTERVAL)} and {formatCount(MAX_INTERVAL)} seconds (one day;
            default is 15 minutes / 900s). The schedule applies from the next cycle.
          </span>
        </label>
        <div className={shared.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy || needsReload}>
            Save
          </ActionButton>
        </div>
      </form>
      {baseline.undeletable ? (
        <div className={styles.defaultPortfolioNotice}>
          <Shield
            size={iconSize.sm}
            className={shared.muted}
            style={{ flexShrink: 0, marginTop: 2 }}
          />
          <div>
            <strong>Initial portfolio</strong>
            <p className={shared.muted}>
              This initial portfolio was created with the account and cannot be deleted.
            </p>
          </div>
        </div>
      ) : (
        <form
          className={`${shared.form} ${styles.dangerCard}`}
          onSubmit={(event) => void onDelete(event)}
        >
          <h2>Delete portfolio</h2>
          <p className={shared.muted}>
            Permanently deletes this portfolio and every Identity, Collection Cycle, Valuation
            Point, and Capital Flow scoped to it. Type the current name to confirm.
          </p>
          <label className={shared.field}>
            <span>Confirm name</span>
            <TextInput
              value={confirmName}
              disabled={busy}
              onChange={(event) => setConfirmName(event.target.value)}
              autoComplete="off"
              required
            />
          </label>
          <div className={shared.actions}>
            <ActionButton
              type="submit"
              tone="danger"
              disabled={busy || needsReload || confirmName !== baseline.name}
            >
              Delete
            </ActionButton>
          </div>
        </form>
      )}
    </>
  );
}
