import { NumericText } from "@/shared/ui/NumericText";
import { type FormEvent, useState } from "react";
import { getCustomValueChange, getHistoryWindow, keys, unwrapOrRedirect } from "@/api";
import type { ValueChange } from "@/api";
import { combinedFlowsCell, valueChangeCell } from "../lib/valueChangeRows";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import { formatUsd } from "@/shared/lib/amount";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { formatInstant } from "@/shared/lib/datetime";
import { boundaryEventCountText, reasonText } from "@/shared/lib/text";
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
          <h2 className={styles.customCalcTitle}>Custom range comparison</h2>
          <p className={styles.customCalcDesc}>
            Calculate value change for a custom time range, adjusted for capital flows and boundary
            events. Times use {timezone ?? "the portfolio timezone"}.
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
      <QueryRecovery query={custom} fallback="The custom value change could not be computed." />
      {range && custom.isPending ? <p className={shared.muted}>Computing comparison.</p> : null}
      {custom.data ? <CustomResult change={custom.data} timezone={timezone} /> : null}
    </section>
  );
}

function CustomResult({ change, timezone }: { change: ValueChange; timezone?: string }) {
  const platforms = usePlatforms();
  return (
    <div className={styles.detailPanel}>
      <div className={styles.pointStatsGrid}>
        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Value change</span>
          <div className={styles.pointStatValue}>
            {valueChangeCell(change.amountUsd, change.startPoint?.totalUsd)}
          </div>
          <span className={styles.pointStatMeta}>
            <span className={shared.nowrap}>{formatInstant(change.from, timezone)}</span> –{" "}
            <span className={shared.nowrap}>{formatInstant(change.to, timezone)}</span>
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Start point</span>
          <div className={styles.pointStatValue}>
            {change.startPoint?.totalUsd ? (
              <NumericText>{formatUsd(change.startPoint.totalUsd)}</NumericText>
            ) : (
              "—"
            )}
          </div>
          <span className={styles.pointStatMeta}>
            {change.startPoint
              ? `at ${formatInstant(change.startPoint.evaluatedAt, timezone)}`
              : "No start point"}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>End point</span>
          <div className={styles.pointStatValue}>
            {change.endPoint?.totalUsd ? (
              <NumericText>{formatUsd(change.endPoint.totalUsd)}</NumericText>
            ) : (
              "—"
            )}
          </div>
          <span className={styles.pointStatMeta}>
            {change.endPoint
              ? `at ${formatInstant(change.endPoint.evaluatedAt, timezone)}`
              : "No end point"}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Flows</span>
          <div className={styles.pointStatValue}>{combinedFlowsCell(change)}</div>
          <span className={styles.pointStatMeta}>
            {!change.startPoint || !change.endPoint
              ? "Boundary events unavailable"
              : boundaryEventCountText(change.boundaryEvents.length)}
          </span>
        </div>
      </div>

      {change.completeness !== "complete" || change.reasons.length > 0 ? (
        <div className={shared.metaStack}>
          {change.completeness !== "complete" ? (
            <div>
              <span className={shared.cellMeta}>Completeness</span>
              <div>
                <CompletenessBadge value={change.completeness} silentWhenHealthy={false} />
              </div>
            </div>
          ) : null}
          {change.reasons.length > 0 ? (
            <p className={shared.notice}>
              Limitations: {change.reasons.map(reasonText).join(", ")}.
            </p>
          ) : null}
        </div>
      ) : null}

      {change.boundaryEvents.length > 0 ? (
        <>
          <h3>Boundary events</h3>
          <DataTable
            caption="Boundary events in the range"
            columns={BOUNDARY_COLUMNS}
            rows={change.boundaryEvents.map((item) =>
              boundaryEventRow(item, timezone, platforms.data?.items),
            )}
            empty="No account entered or left the boundary in this range."
          />
        </>
      ) : null}
    </div>
  );
}
