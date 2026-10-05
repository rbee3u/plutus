import { NumericText } from "@/shared/ui/NumericText";
import { type FormEvent, useState } from "react";
import { getCustomValueChange, getHistoryWindow, keys, unwrapOrRedirect } from "@/api";
import type { ValueChange } from "@/api";
import { valueChangeCell } from "../lib/valueChangeRows";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import { formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { reasonText } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

interface CustomRange {
  from: string;
  to: string;
}

/** Custom Period Comparison: two arbitrary local times isolate capital flows and boundary events. */
export function CustomPeriodPanel({
  portfolioId,
  redirect,
  timezone,
}: {
  portfolioId: string;
  redirect: () => void;
  timezone?: string;
}) {
  const [customFrom, setCustomFrom] = useState("");
  const [customTo, setCustomTo] = useState("");
  const [customError, setCustomError] = useState<string | null>(null);
  const [range, setRange] = useState<CustomRange | null>(null);
  const custom = usePortfolioQuery<ValueChange>({
    key: (userId, portfolioId) =>
      keys.customValueChange(userId, portfolioId, range?.from ?? "", range?.to ?? ""),
    ready: Boolean(range),
    load: (portfolioId) =>
      getCustomValueChange({
        path: { portfolioId },
        query: { from: range?.from ?? "", to: range?.to ?? "" },
      }),
  });

  async function submitCustom(event: FormEvent) {
    event.preventDefault();
    setCustomError(null);
    try {
      const next = unwrapOrRedirect(
        await getHistoryWindow({
          path: { portfolioId },
          query: { fromLocal: customFrom, toLocal: customTo },
        }),
        redirect,
      );
      setRange(next);
    } catch (cause) {
      setCustomError(cause instanceof Error ? cause.message : "Enter a valid date and time.");
    }
  }

  return (
    <section className={styles.customCalcPanel}>
      <div className={styles.customCalcHeader}>
        <div>
          <h2 className={styles.customCalcTitle}>Custom Period Comparison</h2>
          <p className={styles.customCalcDesc}>
            Compare arbitrary valuation points and isolate capital flows and boundary events between
            them. Dates use {timezone ?? "the Portfolio timezone"}.
          </p>
        </div>
      </div>
      <form
        className={styles.rangeForm}
        onSubmit={(event) => {
          void submitCustom(event);
        }}
      >
        <label className={styles.rangeField}>
          <span className={styles.rangeLabel}>From</span>
          <TextInput
            type="datetime-local"
            step="1"
            value={customFrom}
            onChange={(event) => setCustomFrom(event.target.value)}
            required
          />
        </label>
        <label className={styles.rangeField}>
          <span className={styles.rangeLabel}>To</span>
          <TextInput
            type="datetime-local"
            step="1"
            value={customTo}
            onChange={(event) => setCustomTo(event.target.value)}
            required
          />
        </label>
        <div className={styles.rangeActions}>
          <ActionButton type="submit" tone="primary">
            Compute
          </ActionButton>
        </div>
      </form>
      {customError ? (
        <p className={shared.alert} role="alert">
          {customError}
        </p>
      ) : null}
      <QueryRecovery
        error={custom.error}
        hasData={Boolean(custom.data)}
        busy={custom.isFetching}
        fallback="The custom Value Change could not be computed."
        onRetry={() => {
          void custom.refetch();
        }}
      />
      {range && custom.isPending ? <p className={shared.muted}>Computing comparison.</p> : null}
      {custom.data ? <CustomResult change={custom.data} timezone={timezone} /> : null}
    </section>
  );
}

function CustomResult({ change, timezone }: { change: ValueChange; timezone?: string }) {
  return (
    <div className={styles.detailPanel}>
      <dl className={shared.metaList}>
        <div>
          <dt>Range</dt>
          <dd>
            <span className={shared.nowrap}>{formatInstant(change.from, timezone)}</span> –{" "}
            <span className={shared.nowrap}>{formatInstant(change.to, timezone)}</span>
          </dd>
        </div>
        <div>
          <dt>Value change</dt>
          <dd>{valueChangeCell(change.amountUsd)}</dd>
        </div>
        <div>
          <dt>Start point</dt>
          <dd>
            <PointAt point={change.startPoint} timezone={timezone} />
          </dd>
        </div>
        <div>
          <dt>End point</dt>
          <dd>
            <PointAt point={change.endPoint} timezone={timezone} />
          </dd>
        </div>
        <div>
          <dt>Contributions</dt>
          <dd>
            <NumericText>{formatUsd(change.contributionsUsd)}</NumericText>
          </dd>
        </div>
        <div>
          <dt>Withdrawals</dt>
          <dd>
            <NumericText>{formatUsd(change.withdrawalsUsd)}</NumericText>
          </dd>
        </div>
        <div>
          <dt>Boundary in / out</dt>
          <dd>
            <NumericText>{formatUsd(change.boundaryContributionsUsd)}</NumericText> /{" "}
            <NumericText>{formatUsd(change.boundaryWithdrawalsUsd)}</NumericText>
          </dd>
        </div>
        <div>
          <dt>Completeness</dt>
          <dd>
            <CompletenessBadge value={change.completeness} />
          </dd>
        </div>
      </dl>
      {change.reasons.length > 0 ? (
        <p className={shared.notice}>Limitations: {change.reasons.map(reasonText).join(", ")}.</p>
      ) : null}
      <DataTable
        caption="Boundary Events in the range"
        columns={BOUNDARY_COLUMNS}
        rows={change.boundaryEvents.map((item) => boundaryEventRow(item, timezone))}
        empty="No account entered or left the boundary in this range."
      />
    </div>
  );
}

/** A point's total and instant; a missing point is an unknown value, and the timestamp never breaks. */
function PointAt({ point, timezone }: { point: ValueChange["startPoint"]; timezone?: string }) {
  if (!point) {
    return "—";
  }
  return (
    <>
      <NumericText>{formatUsd(point.totalUsd)}</NumericText> at{" "}
      <span className={shared.nowrap}>{formatInstant(point.evaluatedAt, timezone)}</span>
    </>
  );
}
