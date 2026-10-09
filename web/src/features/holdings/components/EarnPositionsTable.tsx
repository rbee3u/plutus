import type { EarnAmount, EarnBalanceSummary, EarnPositionRow, EarnRate, EarnReward } from "@/api";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { formatDecimal, formatPercent } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { DataTable } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import { holdingsAccountColumns, HoldingsFilterHeader } from "./HoldingsFilterHeader";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import { ObservedCell } from "./ObservedCell";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

const TYPE_LABELS: Record<string, string> = {
  flexible: "Flexible",
  fixed: "Fixed",
  bonded: "Bonded",
  timed: "Timed",
  on_chain: "On-chain",
};
const REWARD_LABELS: Record<string, string> = {
  cumulative: "Cumulative",
  pending: "Pending",
  estimated: "Estimated",
  reported: "Reported rewards",
  yesterday: "Yesterday",
  next_payment: "Next payment",
  cumulative_realtime: "Cumulative real-time",
  cumulative_bonus: "Cumulative bonus",
  cumulative_boost: "Cumulative boost",
};

function assetLabel(asset: EarnAmount): string {
  return asset.assetSymbol ?? asset.platformAssetSymbol;
}
function amountLines(assets: EarnAmount[]) {
  return assets.length
    ? assets.map((asset, index) => (
        <div key={`${asset.platformAssetSymbol}-${index}`}>
          <NumericText>{`${formatDecimal(asset.amount)} ${assetLabel(asset)}`}</NumericText>
        </div>
      ))
    : "—";
}
function rateText(rate: EarnRate): string {
  const range =
    rate.upper && rate.upper !== rate.value
      ? `${formatPercent(rate.value)}–${formatPercent(rate.upper)}`
      : formatPercent(rate.value);
  return `${rate.qualifier === "estimated" ? "Est. " : ""}${range} ${rate.kind.toUpperCase()}${rate.platformAssetSymbol ? ` ${rate.platformAssetSymbol}` : ""}`;
}
function rewardLine(reward: EarnReward, index: number) {
  return (
    <div key={`${reward.kind}-${reward.platformAssetSymbol}-${index}`}>
      {REWARD_LABELS[reward.kind] ?? reward.kind}{" "}
      <NumericText>{`${formatDecimal(reward.amount)} ${reward.platformAssetSymbol}`}</NumericText>
    </div>
  );
}

export function EarnPositionsTable({
  items,
  timezone,
  showAccount = false,
  loading = false,
  empty,
  filters,
}: {
  items: EarnPositionRow[];
  timezone?: string;
  showAccount?: boolean;
  loading?: boolean;
  empty?: string;
  filters?: HoldingsFilters;
}) {
  const platforms = usePlatforms();
  return (
    <DataTable
      caption="Earn positions"
      columns={[
        ...(showAccount ? holdingsAccountColumns(filters) : []),
        {
          key: "asset",
          header: "Asset",
          nowrap: true,
          headerContent: filters ? (
            <HoldingsFilterHeader label="Asset" filterKey="symbol" filters={filters} />
          ) : undefined,
        },
        { key: "type", header: "Type", nowrap: true },
        { key: "maturity", header: "Maturity", nowrap: true },
        { key: "rate", header: "Rate", numeric: true, nowrap: true },
        { key: "amount", header: "Amount", numeric: true, nowrap: true },
        { key: "rewards", header: "Rewards", numeric: true, nowrap: true },
        { key: "subscribed", header: "Subscribed", nowrap: true },
        { key: "status", header: "Status", nowrap: true },
        { key: "issues", header: "Issues", nowrap: true },
      ]}
      rows={items.map((item) => {
        const name = `${item.assets.map(assetLabel).join(" / ")} ${item.platformPositionRef}`;
        const displayedRates = item.rates.filter(
          (rate) => rate.qualifier === "reported" || rate.qualifier === "estimated",
        );
        const displayedRewards = item.rewards.filter((reward) =>
          ["cumulative", "pending", "estimated"].includes(reward.kind),
        );
        return {
          id: `${item.accountId}-${item.platformPositionRef}`,
          platform: platformLabel(platforms.data?.items, item.platform),
          identityName: item.identityName,
          asset: item.assets.map((asset, index) => (
            <div
              key={`${asset.platformAssetSymbol}-${index}`}
              title={
                asset.assetSymbol && asset.assetSymbol !== asset.platformAssetSymbol
                  ? `Platform asset: ${asset.platformAssetSymbol}`
                  : undefined
              }
            >
              {assetLabel(asset)}
            </div>
          )),
          type: (
            <HoverDetails
              label={TYPE_LABELS[item.type] ?? "—"}
              name={`Product details for ${name}`}
            >
              <dl>
                <div>
                  <dt>Product</dt>
                  <dd>{item.productName ?? "—"}</dd>
                </div>
                <div>
                  <dt>Product reference</dt>
                  <dd>{item.productRef ?? "—"}</dd>
                </div>
                <div>
                  <dt>Position reference</dt>
                  <dd>{item.platformPositionRef}</dd>
                </div>
                <div>
                  <dt>Row represents</dt>
                  <dd>
                    {item.granularity === "position"
                      ? "Individual position"
                      : item.granularity === "strategy"
                        ? "Strategy aggregate"
                        : "Product aggregate"}
                  </dd>
                </div>
                {item.details.map((detail, index) => (
                  <div key={`${detail.label}-${index}`}>
                    <dt>{detail.label}</dt>
                    <dd>
                      {detail.at ? formatInstant(detail.at, timezone) : (detail.value ?? "—")}
                    </dd>
                  </div>
                ))}
              </dl>
            </HoverDetails>
          ),
          maturity: (
            <span title="Reported term end; principal delivery may occur later. Missing dates are not inferred from duration.">
              {formatInstant(item.maturityAt, timezone)}
            </span>
          ),
          rate: (
            <HoverDetails
              label={
                displayedRates.length
                  ? displayedRates.map((rate, index) => (
                      <div key={index}>
                        <NumericText>{rateText(rate)}</NumericText>
                      </div>
                    ))
                  : "—"
              }
              name={`Rate details for ${name}`}
            >
              {item.rates.length ? (
                item.rates.map((rate, index) => (
                  <p key={index}>
                    {rate.label}: <NumericText>{rateText(rate)}</NumericText>
                  </p>
                ))
              ) : (
                <p>The Platform did not supply a comparable return rate.</p>
              )}
              {item.details
                .filter((detail) => detail.label === "Rate convention")
                .map((detail) => (
                  <p key={detail.label}>{detail.value}</p>
                ))}
            </HoverDetails>
          ),
          amount: (
            <HoverDetails label={amountLines(item.assets)} name={`Amount details for ${name}`}>
              <p>
                {item.amountBasis === "principal"
                  ? "Reported principal"
                  : item.amountBasis === "allocated"
                    ? "Total allocated amount"
                    : "Total holding amount"}
                . Already represented in Balance equity.
              </p>
            </HoverDetails>
          ),
          rewards: (
            <HoverDetails
              label={displayedRewards.length ? displayedRewards.map(rewardLine) : "—"}
              name={`Reward details for ${name}`}
            >
              {item.rewards.length ? (
                item.rewards.map(rewardLine)
              ) : (
                <p>Reward amounts were not supplied.</p>
              )}
              <p>
                Reward measures have separate scopes and are not added together or added to Account
                value.
              </p>
              {item.details
                .filter((detail) => detail.label === "Cumulative reward scope")
                .map((detail) => (
                  <p key={detail.label}>{detail.value}</p>
                ))}
            </HoverDetails>
          ),
          subscribed: formatInstant(item.subscribedAt, timezone),
          status: item.status ?? "—",
          issues: (
            <ObservedCell
              observedAt={item.reportObservedAt}
              freshness={item.freshness}
              issues={item.issues}
              name={name}
              timezone={timezone}
            />
          ),
        };
      })}
      empty={
        loading ? "Loading Earn positions." : (empty ?? "No Earn positions match this selection.")
      }
    />
  );
}

export function EarnBalanceSummaries({
  items,
  timezone,
  showAccount = false,
}: {
  items: EarnBalanceSummary[];
  timezone?: string;
  showAccount?: boolean;
}) {
  const platforms = usePlatforms();
  if (!items.length) return null;
  return (
    <section>
      <h3 className={styles.accountSectionTitle}>Known balances without product details</h3>
      <p className={shared.muted}>
        Product details were not collected in these source reports. These are Balance summaries, not
        product positions. A successful Collection will supply new details.
      </p>
      <DataTable
        caption="Earn Balance summaries without product details"
        columns={[
          ...(showAccount
            ? [
                { key: "platform", header: "Platform", nowrap: true },
                { key: "identityName", header: "Identity", nowrap: true },
              ]
            : []),
          { key: "asset", header: "Asset", nowrap: true },
          { key: "amount", header: "Amount", numeric: true, nowrap: true },
          { key: "issues", header: "Issues", nowrap: true },
        ]}
        rows={items.map((item) => ({
          id: item.accountId,
          platform: platformLabel(platforms.data?.items, item.platform),
          identityName: item.identityName,
          asset: item.assets.length
            ? item.assets.map((asset, index) => <div key={index}>{assetLabel(asset)}</div>)
            : "—",
          amount: amountLines(item.assets),
          issues: (
            <ObservedCell
              observedAt={item.reportObservedAt}
              freshness={item.freshness}
              issues={item.issues}
              name={`${item.identityName} Earn Balance summary`}
              timezone={timezone}
            />
          ),
        }))}
        empty="No Balance summaries."
      />
    </section>
  );
}
