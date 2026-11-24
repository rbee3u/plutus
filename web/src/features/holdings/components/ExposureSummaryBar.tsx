import { useState } from "react";
import { AlertTriangle } from "lucide-react";
import type { ExposureCoverage, ExposureSummary, ReadContext } from "@/api";
import { formatCount, formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { iconSize } from "@/shared/icons/iconSizes";
import { Badge, CompletenessBadge, FreshnessBadge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import shared from "@/shared/styles/page.module.css";
import { ExposureHowItWorks } from "./ExposureHowItWorks";
import styles from "../styles.module.css";

function plural(count: number, singular: string, pluralForm: string) {
  return `${formatCount(count)} ${count === 1 ? singular : pluralForm}`;
}

function symbolList(symbols: string[], limit?: number) {
  const shown = limit == null ? symbols : symbols.slice(0, limit);
  const suffix = limit != null && symbols.length > limit ? `, +${symbols.length - limit} more` : "";
  return `${shown.join(", ")}${suffix}`;
}

function valueReasons(summary: ExposureSummary, coverage: ExposureCoverage) {
  const reasons: string[] = [];
  if (summary.unpricedAssetCount > 0) {
    reasons.push(
      `${plural(summary.unpricedAssetCount, "asset", "assets")} without a USD value (${symbolList(
        summary.unpricedAssetSymbols,
        3,
      )})`,
    );
  }
  if (summary.partialCount > 0) {
    reasons.push(plural(summary.partialCount, "partial row", "partial rows"));
  }
  if (coverage.missingAccountCount > 0) {
    reasons.push(`${plural(coverage.missingAccountCount, "account", "accounts")} missing`);
  }
  if (coverage.unattributedPositionCount > 0) {
    reasons.push(
      `${plural(coverage.unattributedPositionCount, "position", "positions")} unattributed`,
    );
  }
  return reasons;
}

function ExposureValue({ value }: { value: string | null }) {
  if (value == null) {
    return (
      <span className={shared.muted}>
        <span aria-hidden="true">—</span>
        <span className={shared.srOnly}>Unknown</span>
      </span>
    );
  }
  return <NumericText>{formatUsd(value)}</NumericText>;
}

function ValueNote({
  summary,
  coverage,
}: {
  summary: ExposureSummary;
  coverage: ExposureCoverage;
}) {
  const reasons = valueReasons(summary, coverage);
  if (reasons.length === 0) return null;
  const text = `USD value is partial · ${reasons.join(" · ")}`;
  return (
    <HoverDetails
      className={styles.exposureValueNote}
      label={
        <>
          <AlertTriangle size={iconSize.sm} aria-hidden="true" />
          <span>{text}</span>
        </>
      }
      name={`USD value details: ${reasons.join("; ")}`}
      preferredPlacement="below"
    >
      {summary.unpricedAssetCount > 0 ? (
        <p>No USD value: {symbolList(summary.unpricedAssetSymbols)}.</p>
      ) : null}
      {summary.partialCount > 0 ? (
        <p>Partial rows sum the inputs they know; the table names the unknown inputs.</p>
      ) : null}
      {coverage.missingAccountCount > 0 ? (
        <p>
          Missing accounts have no compatible successful report; their holdings are not counted.
        </p>
      ) : null}
      {coverage.unattributedPositionCount > 0 ? (
        <p>Unattributed positions have no catalog base asset and are not counted.</p>
      ) : null}
    </HoverDetails>
  );
}

/** Portfolio-level counts and the signed USD composition instead of a headline numeral. */
export function ExposureSummaryBar({
  context,
  summary,
  coverage,
  filtered,
}: {
  context: ReadContext;
  summary: ExposureSummary;
  coverage: ExposureCoverage;
  filtered: boolean;
}) {
  const [help, setHelp] = useState(false);
  const counts = [
    `${formatCount(summary.assetCount)} ${summary.assetCount === 1 ? "asset" : "assets"}`,
  ];
  if (summary.longCount > 0) counts.push(`${formatCount(summary.longCount)} long`);
  if (summary.shortCount > 0) counts.push(`${formatCount(summary.shortCount)} short`);
  if (summary.flatCount > 0) counts.push(`${formatCount(summary.flatCount)} flat`);
  return (
    <>
      <div className={`${styles.holdingsSummaryBar} ${styles.exposureSummary}`}>
        <div className={styles.exposureSummaryMain}>
          <div className={styles.exposureSummaryHead}>
            <span className={styles.holdingsSummaryLabel}>
              {filtered ? "FILTERED EXPOSURE" : "PORTFOLIO EXPOSURE"}
            </span>
            <span className={styles.exposureSummaryCounts}>{counts.join(" · ")}</span>
            {summary.partialCount > 0 ? (
              <Badge tone="warn">{formatCount(summary.partialCount)} partial</Badge>
            ) : null}
            {summary.unknownCount > 0 ? (
              <Badge tone="warn">{formatCount(summary.unknownCount)} unknown</Badge>
            ) : null}
            <CompletenessBadge value={summary.completeness} />
            <FreshnessBadge value={summary.freshness} />
          </div>
          <dl className={styles.exposureValues}>
            <div>
              <dt>Stablecoins</dt>
              <dd>
                <ExposureValue value={summary.stablecoinValueUsd} />
              </dd>
            </div>
            <div>
              <dt>Non-stablecoins</dt>
              <dd>
                <ExposureValue value={summary.nonStablecoinValueUsd} />
              </dd>
            </div>
            <div>
              <dt>Total</dt>
              <dd className={styles.exposureValueTotal}>
                <ExposureValue value={summary.totalValueUsd} />
              </dd>
            </div>
          </dl>
          <ValueNote summary={summary} coverage={coverage} />
        </div>
        <div className={`${styles.holdingsSummaryEnd} ${styles.exposureSummaryEnd}`}>
          <div className={styles.holdingsSummaryMeta}>
            <span>
              {context.evaluatedAt
                ? `Evaluated ${formatInstant(context.evaluatedAt, context.timezone)} · ${context.timezone}`
                : `Times in ${context.timezone}`}
            </span>
          </div>
          <button type="button" className={styles.exposureHelpButton} onClick={() => setHelp(true)}>
            How it works
          </button>
        </div>
      </div>
      <ExposureHowItWorks
        open={help}
        onClose={() => setHelp(false)}
        stablecoinSymbols={summary.stablecoinSymbols}
      />
    </>
  );
}
