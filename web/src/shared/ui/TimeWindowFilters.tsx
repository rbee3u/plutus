import { useState, type FormEvent } from "react";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput } from "@/shared/ui/FormControls";
import { RequestRecovery } from "@/shared/ui/QueryRecovery";
import {
  TIME_WINDOW_PRESETS,
  timeWindowLocalValue,
  type TimeWindowPreset,
} from "@/shared/lib/timeWindow";
import styles from "./TimeWindowFilters.module.css";
import shared from "@/shared/styles/page.module.css";

export interface TimeWindowState {
  preset: TimeWindowPreset | null;
  window?: {
    from: string;
    to: string;
    context: {
      timezone: string;
      portfolioId?: string;
    };
  };
  applying: boolean;
  applyError: Error | null;
  error: unknown;
  busy: boolean;
  selectPreset: (preset: TimeWindowPreset) => void;
  apply: (fromLocal: string, toLocal: string) => Promise<void>;
  retry: () => void;
}

export function TimeWindowFilters({
  state,
  ariaLabel = "Time range",
}: {
  state: TimeWindowState;
  ariaLabel?: string;
}) {
  return (
    <div className={styles.timeWindowToolbar}>
      <div className={styles.timeWindowPresets} role="group" aria-label={ariaLabel}>
        {TIME_WINDOW_PRESETS.map((preset) => (
          <ActionButton
            key={preset}
            type="button"
            tone={state.preset === preset ? "primary" : "ghost"}
            aria-pressed={state.preset === preset}
            disabled={state.applying}
            onClick={() => state.selectPreset(preset)}
            className={state.preset === preset ? styles.presetActive : styles.presetInactive}
          >
            {preset}
          </ActionButton>
        ))}
      </div>
      {state.window ? (
        <DateRangeForm key={`${state.window.from}|${state.window.to}`} state={state} />
      ) : null}
      {state.applyError ? (
        <p className={shared.alert} role="alert">
          {state.applyError.message}
        </p>
      ) : null}
      <RequestRecovery
        error={state.error}
        hasData={Boolean(state.window)}
        busy={state.busy || state.applying}
        fallback="The time window could not be loaded."
        onRetry={state.retry}
      />
      {state.busy && !state.window ? <p className={shared.muted}>Loading time window.</p> : null}
    </div>
  );
}

function DateRangeForm({ state }: { state: TimeWindowState }) {
  const window = state.window!;
  const timezone = window.context.timezone;
  const [from, setFrom] = useState(() => timeWindowLocalValue(window.from, timezone));
  const [to, setTo] = useState(() => timeWindowLocalValue(window.to, timezone));

  function submit(event: FormEvent) {
    event.preventDefault();
    void state.apply(from, to);
  }

  return (
    <form className={styles.rangeForm} onSubmit={submit}>
      <label className={styles.rangeField}>
        <span className={styles.rangeLabel}>From</span>
        <TextInput
          type="datetime-local"
          step="1"
          required
          value={from}
          onChange={(event) => setFrom(event.target.value)}
        />
      </label>
      <label className={styles.rangeField}>
        <span className={styles.rangeLabel}>To</span>
        <TextInput
          type="datetime-local"
          step="1"
          required
          value={to}
          onChange={(event) => setTo(event.target.value)}
        />
      </label>
      <div className={styles.rangeActions}>
        <ActionButton type="submit" tone="primary" disabled={state.applying}>
          {state.applying ? "Applying" : "Apply"}
        </ActionButton>
        <span className={styles.rangeTimezone}>Timezone: {timezone}</span>
      </div>
    </form>
  );
}
