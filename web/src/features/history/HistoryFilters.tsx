import { useState, type FormEvent } from "react";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput, Select } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Pager } from "@/shared/ui/Pager";
import {
  HISTORY_PRESETS,
  HISTORY_PAGE_SIZES,
  historyLocalValue,
  type useHistoryWindow,
  type useHistoryPager,
} from "./useHistoryWindow";
import styles from "@/features/pages.module.css";

type WindowState = ReturnType<typeof useHistoryWindow>;
export function HistoryFilters({ state }: { state: WindowState }) {
  return (
    <>
      <div className={styles.actions} role="group" aria-label="History time range">
        {HISTORY_PRESETS.map((preset) => (
          <ActionButton
            key={preset}
            type="button"
            tone={state.preset === preset ? "primary" : "ghost"}
            aria-pressed={state.preset === preset}
            disabled={state.applying}
            onClick={() => state.selectPreset(preset)}
          >
            {preset}
          </ActionButton>
        ))}
      </div>
      {state.window ? (
        <DateRangeForm key={`${state.window.from}|${state.window.to}`} state={state} />
      ) : null}
      {state.applyError ? (
        <p className={styles.alert} role="alert">
          {state.applyError.message}
        </p>
      ) : null}
      <QueryRecovery
        error={state.error}
        hasData={Boolean(state.window)}
        busy={state.busy || state.applying}
        fallback="The history window could not be loaded."
        onRetry={state.retry}
      />
      {state.busy && !state.window ? <p className={styles.muted}>Loading history window.</p> : null}
    </>
  );
}

function DateRangeForm({ state }: { state: WindowState }) {
  const window = state.window!;
  const timezone = window.context.timezone;
  const [from, setFrom] = useState(() => historyLocalValue(window.from, timezone));
  const [to, setTo] = useState(() => historyLocalValue(window.to, timezone));
  function submit(event: FormEvent) {
    event.preventDefault();
    void state.apply(from, to);
  }
  return (
    <form className={styles.rangeForm} onSubmit={submit}>
      <label className={styles.field}>
        <span>From</span>
        <TextInput
          type="datetime-local"
          step="1"
          required
          value={from}
          onChange={(event) => setFrom(event.target.value)}
        />
      </label>
      <label className={styles.field}>
        <span>To</span>
        <TextInput
          type="datetime-local"
          step="1"
          required
          value={to}
          onChange={(event) => setTo(event.target.value)}
        />
      </label>
      <ActionButton type="submit" tone="primary" disabled={state.applying}>
        {state.applying ? "Applying" : "Apply"}
      </ActionButton>
      <span className={styles.muted}>Timezone: {timezone}</span>
    </form>
  );
}

export function HistoryPager({
  pager,
  total,
  label,
  busy = false,
}: {
  pager: ReturnType<typeof useHistoryPager>;
  total: number;
  label: string;
  busy?: boolean;
}) {
  return (
    <div className={styles.headingRow} aria-label={`${label} pagination`}>
      <label className={`${styles.field} ${styles.toolbarField}`}>
        <span>Rows per page</span>
        <Select
          aria-label={`${label} rows per page`}
          value={pager.limit}
          disabled={busy}
          onChange={(event) => pager.onLimitChange(Number(event.target.value))}
        >
          {HISTORY_PAGE_SIZES.map((size) => (
            <option key={size} value={size}>
              {size}
            </option>
          ))}
        </Select>
      </label>
      <Pager
        offset={pager.offset}
        limit={pager.limit}
        total={total}
        onChange={pager.onChange}
        busy={busy}
      />
    </div>
  );
}
