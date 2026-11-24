import { getExposureDetail, keys } from "@/api";
import type { ExposureContribution, ExposureDetail } from "@/api";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { Badge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

function contributionState(c: ExposureContribution, timezone?: string) {
  if (c.state === "unknown") {
    return (
      <HoverDetails
        label={<Badge tone="warn">Unknown</Badge>}
        name={`Unknown ${c.assetSymbol} contribution`}
      >
        {c.issues.length > 0 ? (
          c.issues.map((issue, index) => <p key={`${issue.code}-${index}`}>{issue.detail}</p>)
        ) : (
          <p>The contribution cannot be determined from the retained report.</p>
        )}
      </HoverDetails>
    );
  }
  if (c.freshness === "stale") {
    return (
      <HoverDetails
        label={<Badge tone="warn">Stale</Badge>}
        name={`Stale ${c.assetSymbol} contribution`}
      >
        <p>An earlier successful account report is carried forward.</p>
        {c.reportObservedAt ? (
          <p className={shared.cellMeta}>Observed {formatInstant(c.reportObservedAt, timezone)}</p>
        ) : null}
      </HoverDetails>
    );
  }
  return <span className={shared.muted}>—</span>;
}

function accountLabel(detail: ExposureDetail, c: ExposureContribution) {
  const mapped = c.assetSymbol && c.assetSymbol !== detail.canonicalAssetSymbol;
  return (
    <span className={shared.cellStack}>
      <span>
        {c.accountName}
        {c.contractOnly ? (
          <span className={styles.exposureContractOnly}>
            <Badge tone="neutral">Contract only</Badge>
          </span>
        ) : null}
      </span>
      {mapped ? <span className={shared.cellMeta}>{c.assetSymbol}</span> : null}
    </span>
  );
}

function ContributionTable({ detail, timezone }: { detail: ExposureDetail; timezone?: string }) {
  const platforms = new Set(detail.contributions.map((item) => item.platform));
  return (
    <div className={styles.exposureContribWrap}>
      <div className={styles.exposureContribHead}>
        <span className={styles.exposureContribTitle}>Contributions</span>
        <span className={styles.exposureContribMeta}>
          {detail.accountCount} {detail.accountCount === 1 ? "account" : "accounts"} ·{" "}
          {platforms.size} {platforms.size === 1 ? "platform" : "platforms"}
        </span>
      </div>
      {detail.coverage.missingAccountCount > 0 || detail.coverage.unattributedPositionCount > 0 ? (
        <p className={shared.muted}>
          {detail.coverage.missingAccountCount > 0
            ? `${detail.coverage.missingAccountCount} account${detail.coverage.missingAccountCount === 1 ? "" : "s"} missing`
            : ""}
          {detail.coverage.missingAccountCount > 0 && detail.coverage.unattributedPositionCount > 0
            ? " · "
            : ""}
          {detail.coverage.unattributedPositionCount > 0
            ? `${detail.coverage.unattributedPositionCount} unattributed position${detail.coverage.unattributedPositionCount === 1 ? "" : "s"}`
            : ""}
          . This total may omit their holdings.
        </p>
      ) : null}
      <table className={styles.exposureContribTable}>
        <thead>
          <tr>
            <th>Platform</th>
            <th>Identity</th>
            <th>Account</th>
            <th className={styles.exposureContribNum}>Contribution</th>
            <th>State</th>
          </tr>
        </thead>
        <tbody>
          {detail.contributions.map((c) => (
            <tr key={`${c.accountId}-${c.assetSymbol}`}>
              <td>{c.platformLabel}</td>
              <td>{c.identityName}</td>
              <td>{accountLabel(detail, c)}</td>
              <td className={styles.exposureContribNum}>
                {c.quantity == null ? (
                  <span className={shared.muted}>—</span>
                ) : (
                  <NumericText>{formatDecimal(c.quantity)}</NumericText>
                )}
              </td>
              <td>{contributionState(c, timezone)}</td>
            </tr>
          ))}
          <tr className={styles.exposureContribTotal}>
            <td colSpan={3}>Total</td>
            <td className={styles.exposureContribNum}>
              {detail.netQuantity == null ? (
                <span className={shared.muted}>—</span>
              ) : (
                <NumericText>{formatDecimal(detail.netQuantity)}</NumericText>
              )}
            </td>
            <td />
          </tr>
        </tbody>
      </table>
    </div>
  );
}

function ContributionList({ detail, timezone }: { detail: ExposureDetail; timezone?: string }) {
  return (
    <div className={styles.exposureContribList}>
      {detail.contributions.map((c) => (
        <div className={styles.exposureContribRow} key={`${c.accountId}-${c.assetSymbol}`}>
          <div className={styles.exposureContribRowMain}>
            <span>
              {c.platformLabel} · {c.identityName} · {c.accountName}
              {c.assetSymbol && c.assetSymbol !== detail.canonicalAssetSymbol
                ? ` (${c.assetSymbol})`
                : ""}
            </span>
            <span className={styles.exposureContribRowQty}>
              {c.quantity == null ? (
                <span className={shared.muted}>—</span>
              ) : (
                formatDecimal(c.quantity)
              )}
            </span>
          </div>
          <div className={styles.exposureContribRowMeta}>
            {c.contractOnly ? <Badge tone="neutral">Contract only</Badge> : null}
            {c.state !== "complete" || c.freshness === "stale"
              ? contributionState(c, timezone)
              : null}
            {c.freshness === "stale" && c.reportObservedAt ? (
              <span>{formatInstant(c.reportObservedAt, timezone)}</span>
            ) : null}
          </div>
        </div>
      ))}
      <div className={styles.exposureContribListTotal}>
        <span>Total</span>
        <span>
          {detail.netQuantity == null ? (
            <span className={shared.muted}>—</span>
          ) : (
            <NumericText>{formatDecimal(detail.netQuantity)}</NumericText>
          )}
        </span>
      </div>
    </div>
  );
}

export function ExposureContributions({
  symbol,
  cycleId,
  platform,
  identityId,
  timezone,
  compact = false,
}: {
  symbol: string;
  cycleId: string;
  platform?: string;
  identityId?: string;
  timezone?: string;
  compact?: boolean;
}) {
  const query = usePortfolioQuery<ExposureDetail>({
    key: (userId, portfolioId) =>
      keys.exposureDetail(userId, portfolioId, symbol, platform, identityId, cycleId),
    ready: Boolean(cycleId),
    load: (portfolioId) =>
      getExposureDetail({
        path: { portfolioId, canonicalAssetSymbol: symbol },
        query: {
          platform: platform || undefined,
          identityId: identityId || undefined,
          cycleId,
        },
      }),
  });
  if (query.isPending) {
    return <p className={shared.muted}>Loading contributions…</p>;
  }
  if (query.isError && !query.data) {
    return <QueryRecovery query={query} fallback="Contributions could not be loaded." />;
  }
  const detail = query.data;
  if (!detail) {
    return <p className={shared.muted}>No contributions are available for this asset.</p>;
  }
  return compact ? (
    <ContributionList detail={detail} timezone={timezone} />
  ) : (
    <ContributionTable detail={detail} timezone={timezone} />
  );
}
