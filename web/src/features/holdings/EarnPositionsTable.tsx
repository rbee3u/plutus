import type {
  EarnAmount,
  EarnBalanceSummary,
  EarnPositionRow,
  EarnRate,
  EarnReward,
} from "@/api/generated/types.gen";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { formatDecimal, formatPercent } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { DataTable } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import { HoldingsFilterHeader } from "./HoldingsFilterHeader";
import type { HoldingsFilters } from "./useHoldingsFilters";
import { ObservedCell } from "./ObservedCell";
import styles from "@/features/pages.module.css";

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

function assetLabel(a: EarnAmount): string {
  return a.assetSymbol ?? a.symbol;
}
function amountLines(assets: EarnAmount[]) {
  return assets.length
    ? assets.map((a, i) => (
        <div key={`${a.symbol}-${i}`}>
          <NumericText>{`${formatDecimal(a.amount)} ${assetLabel(a)}`}</NumericText>
        </div>
      ))
    : "—";
}
function rateText(r: EarnRate): string {
  const range =
    r.upper && r.upper !== r.value
      ? `${formatPercent(r.value)}–${formatPercent(r.upper)}`
      : formatPercent(r.value);
  return `${r.qualifier === "estimated" ? "Est. " : ""}${range} ${r.kind.toUpperCase()}${r.symbol ? ` ${r.symbol}` : ""}`;
}
function rewardLine(r: EarnReward, i: number) {
  return (
    <div key={`${r.kind}-${r.symbol}-${i}`}>
      {REWARD_LABELS[r.kind] ?? r.kind}{" "}
      <NumericText>{`${formatDecimal(r.amount)} ${r.symbol}`}</NumericText>
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
        ...(showAccount
          ? [
              {
                key: "platform",
                header: "Platform",
                nowrap: true,
                headerContent: filters ? (
                  <HoldingsFilterHeader label="Platform" filterKey="platform" filters={filters} />
                ) : undefined,
              },
              {
                key: "identity",
                header: "Identity",
                nowrap: true,
                headerContent: filters ? (
                  <HoldingsFilterHeader label="Identity" filterKey="identityId" filters={filters} />
                ) : undefined,
              },
            ]
          : []),
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
        { key: "observed", header: "Observed", nowrap: true },
      ]}
      rows={items.map((item) => {
        const name = `${item.assets.map(assetLabel).join(" / ")} ${item.platformPositionRef}`;
        const displayedRates = item.rates.filter(
          (r) => r.qualifier === "reported" || r.qualifier === "estimated",
        );
        const displayedRewards = item.rewards.filter((r) =>
          ["cumulative", "pending", "estimated"].includes(r.kind),
        );
        return {
          id: `${item.accountId}-${item.platformPositionRef}`,
          platform: platformLabel(platforms.data?.items, item.platform),
          identity: item.identityName,
          asset: item.assets.map((a, i) => (
            <div
              key={`${a.symbol}-${i}`}
              title={
                a.assetSymbol && a.assetSymbol !== a.symbol
                  ? `Platform listing: ${a.symbol}`
                  : undefined
              }
            >
              {assetLabel(a)}
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
                {item.details.map((detail, i) => (
                  <div key={`${detail.label}-${i}`}>
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
                  ? displayedRates.map((r, i) => (
                      <div key={i}>
                        <NumericText>{rateText(r)}</NumericText>
                      </div>
                    ))
                  : "—"
              }
              name={`Rate details for ${name}`}
            >
              {item.rates.length ? (
                item.rates.map((r, i) => (
                  <p key={i}>
                    {r.label}: <NumericText>{rateText(r)}</NumericText>
                  </p>
                ))
              ) : (
                <p>The Platform did not supply a comparable return rate.</p>
              )}
              {item.details
                .filter((d) => d.label === "Rate convention")
                .map((d) => (
                  <p key={d.label}>{d.value}</p>
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
                .filter((d) => d.label === "Cumulative reward scope")
                .map((d) => (
                  <p key={d.label}>{d.value}</p>
                ))}
            </HoverDetails>
          ),
          subscribed: formatInstant(item.subscribedAt, timezone),
          status: item.status ?? "—",
          observed: (
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
      <p className={styles.muted}>
        Product details were not collected in these source reports. These are Balance summaries, not
        product positions. A successful Collection will supply new details.
      </p>
      <DataTable
        caption="Earn Balance summaries without product details"
        columns={[
          ...(showAccount
            ? [
                { key: "platform", header: "Platform", nowrap: true },
                { key: "identity", header: "Identity", nowrap: true },
              ]
            : []),
          { key: "asset", header: "Asset", nowrap: true },
          { key: "amount", header: "Amount", numeric: true, nowrap: true },
          { key: "observed", header: "Observed", nowrap: true },
        ]}
        rows={items.map((item) => ({
          id: item.accountId,
          platform: platformLabel(platforms.data?.items, item.platform),
          identity: item.identityName,
          asset: item.assets.length
            ? item.assets.map((a, i) => <div key={i}>{assetLabel(a)}</div>)
            : "—",
          amount: amountLines(item.assets),
          observed: (
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
