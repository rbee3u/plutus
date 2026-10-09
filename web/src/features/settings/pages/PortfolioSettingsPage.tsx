import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { portfoliosOptions } from "@/shared/lib/usePortfolios";
import { Shield } from "lucide-react";
import { formatCount } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { useQueryClient } from "@tanstack/react-query";
import { type ChangeEvent, type FormEvent, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { deletePortfolio, keys, patchPortfolio, problemMessage } from "@/api";
import { LAST_PORTFOLIO_KEY } from "@/shared/lib/preferences";
import { type PortfolioView, usePortfolio } from "@/shared/lib/portfolio";
import { storePreference } from "@/shared/lib/preferences";
import { useSessionScope } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Select, TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/settings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

const MIN_INTERVAL = 60;
const MAX_INTERVAL = 86_400;
const DEFAULT_INTERVAL = 900;
const CONFLICT =
  "The portfolio changed in another session. The latest values were reloaded; review and save again.";

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
  { label: "5m", seconds: 300 },
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
  const [notice, setNotice] = useState<string | null>(null);
  const [conflict, setConflict] = useState<string | null>(null);

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
      {conflict ? (
        <p className={shared.alert} role="alert">
          {conflict}
        </p>
      ) : null}
      {notice ? (
        <p className={shared.notice} role="status">
          {notice}
        </p>
      ) : null}
      <QueryRecovery query={portfolio} fallback="The portfolio could not be loaded." />
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
        <p className={shared.muted}>Loading portfolio settings.</p>
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
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const write = useAuthenticatedWrite();
  const navigate = useNavigate();
  const [name, setName] = useState(view.item.name);
  const [timezone, setTimezone] = useState(view.item.timezone);
  const [interval, setInterval] = useState(String(view.item.collectionIntervalSeconds));
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
      setInterval(String(view.item.collectionIntervalSeconds || DEFAULT_INTERVAL));
      return;
    }
    const num = Number(interval);
    if (num < MIN_INTERVAL) {
      setInterval(String(MIN_INTERVAL));
    } else if (num > MAX_INTERVAL) {
      setInterval(String(MAX_INTERVAL));
    }
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
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
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        patchPortfolio({
          path: { portfolioId },
          headers: { "If-Match": view.etag },
          body: {
            name: name.trim(),
            timezone: timezone.trim(),
            collectionIntervalSeconds: seconds,
          },
        }),
      );
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
      const result = await write(() =>
        deletePortfolio({
          path: { portfolioId },
          headers: { "If-Match": view.etag },
        }),
      );
      if (result.response?.status === 412) {
        await onConflict();
        return;
      }
      if (result.error || result.response?.status !== 204) {
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
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
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
                onClick={() => setTimezone(browserTimezone)}
                title={`Set timezone to ${browserTimezone}`}
              >
                Match browser ({browserTimezone})
              </button>
            ) : null}
          </div>
          <Select value={timezone} onChange={(event) => setTimezone(event.target.value)} required>
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
                Min {formatCount(MIN_INTERVAL)}s (1m)
              </span>
            ) : null}
          </div>
          <TextInput
            type="text"
            inputMode="numeric"
            pattern="[0-9]*"
            value={interval}
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
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Save
          </ActionButton>
        </div>
      </form>
      {view.item.undeletable ? (
        <div className={styles.defaultPortfolioNotice}>
          <Shield
            size={iconSize.sm}
            className={shared.muted}
            style={{ flexShrink: 0, marginTop: 2 }}
          />
          <div>
            <strong>Default portfolio</strong>
            <p className={shared.muted}>
              This default portfolio was created with the account and cannot be deleted.
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
              onChange={(event) => setConfirmName(event.target.value)}
              autoComplete="off"
              required
            />
          </label>
          <div className={shared.actions}>
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
