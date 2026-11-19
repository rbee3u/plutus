import { ChevronRight, CircleAlert, Plus, TriangleAlert } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import type { AccountValuation, Overview as Tree } from "@/api";
import { type ManualTarget } from "../dialogs/manual";
import { ManualDialog } from "../dialogs/ManualBalances";
import { IconButton } from "@/shared/ui/IconButton";
import { AccountDetail } from "@/features/holdings/components/AccountDetail";
import { useLeverageRisk } from "../hooks/useLeverageRisk";
import {
  highLeverageDescription,
  type LeverageRiskEntry,
  type LeverageRiskMap,
} from "../lib/leverageRisk";
import { formatAmount, formatPercent, parseDecimal } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { Badge } from "@/shared/ui/Badge";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function HoldingsTree({ portfolioId, tree }: { portfolioId: string; tree: Tree }) {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [manualTarget, setManualTarget] = useState<ManualTarget | null>(null);
  const titleId = useId();
  const totalUsdDecimal = parseDecimal(tree.point?.totalUsd);
  const { risk, isError: riskError } = useLeverageRisk(portfolioId, tree);

  function toggle(key: string) {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <section className={styles.treeCard} aria-labelledby={titleId}>
      <div className={styles.treeCardHeader}>
        <h2 id={titleId} className={styles.treeCardTitle}>
          Holdings tree
        </h2>
      </div>
      {riskError ? (
        <p className={styles.leverageRiskNotice} role="status">
          <CircleAlert size={iconSize.sm} aria-hidden="true" />
          Leverage warnings could not be checked.
        </p>
      ) : null}
      {tree.platforms.length === 0 ? (
        <p className={shared.muted}>No accounts to display.</p>
      ) : (
        <ul
          className={styles.holdingsTree}
          aria-label="Holdings tree at the latest valuation point"
        >
          {tree.platforms.map((platform) => {
            const platformUsdDecimal = parseDecimal(platform.valueUsd);
            const percent =
              totalUsdDecimal && platformUsdDecimal && !totalUsdDecimal.isZero()
                ? platformUsdDecimal.dividedBy(totalUsdDecimal).toFixed()
                : null;
            const platformRisk = riskEntriesForPlatform(platform, risk);
            return (
              <TreeNode
                key={platform.platform}
                name={platform.label}
                value={platform.valueUsd}
                percent={percent}
                riskDescription={riskDescriptionFor("platform", platformRisk)}
                level="platform"
                open={!expanded.has(`platform:${platform.platform}`)}
                onToggle={() => toggle(`platform:${platform.platform}`)}
              >
                <ul className={styles.treeChildren}>
                  {platform.identities.map((identity) => {
                    const identityUsdDecimal = parseDecimal(identity.valueUsd);
                    const identityPercent =
                      totalUsdDecimal && identityUsdDecimal && !totalUsdDecimal.isZero()
                        ? identityUsdDecimal.dividedBy(totalUsdDecimal).toFixed()
                        : null;
                    const identityRisk = riskEntriesForAccounts(identity.accounts, risk);
                    return (
                      <TreeNode
                        key={identity.id}
                        name={identity.displayName}
                        value={identity.valueUsd}
                        percent={identityPercent}
                        riskDescription={riskDescriptionFor("identity", identityRisk)}
                        actions={
                          <IconButton
                            label={`Add manual balance for ${identity.displayName}`}
                            onClick={() => setManualTarget({ identityId: identity.id })}
                          >
                            <Plus size={iconSize.sm} aria-hidden="true" />
                          </IconButton>
                        }
                        level="identity"
                        open={expanded.has(`identity:${identity.id}`)}
                        onToggle={() => toggle(`identity:${identity.id}`)}
                      >
                        <ul className={styles.treeChildren}>
                          {identity.accounts.map((account) => {
                            const accountRisk = risk.get(account.accountId);
                            const accountRiskDescription = accountRisk
                              ? highLeverageDescription("account", [
                                  {
                                    accountName: account.accountName,
                                    display: accountRisk.display,
                                  },
                                ])
                              : undefined;
                            return (
                              <TreeNode
                                key={account.accountId}
                                name={account.accountName}
                                actionName={`balances and positions for ${account.accountName}`}
                                value={account.valueUsd}
                                riskDescription={accountRiskDescription}
                                level="account"
                                open={expanded.has(`account:${account.accountId}`)}
                                onToggle={() => toggle(`account:${account.accountId}`)}
                              >
                                <div
                                  className={`${styles.treeChildren} ${styles.treeAccountDetail}`}
                                >
                                  <AccountDetail
                                    portfolioId={portfolioId}
                                    account={account}
                                    onManual={setManualTarget}
                                    cycleId={tree.context.cycleId ?? ""}
                                    timezone={tree.context.timezone}
                                    highlightLeverageRisk
                                  />
                                </div>
                              </TreeNode>
                            );
                          })}
                        </ul>
                      </TreeNode>
                    );
                  })}
                </ul>
              </TreeNode>
            );
          })}
        </ul>
      )}
      <ManualDialog
        portfolioId={portfolioId}
        target={manualTarget}
        onClose={() => setManualTarget(null)}
      />
    </section>
  );
}

function riskEntriesForAccounts(
  accounts: ReadonlyArray<AccountValuation>,
  risk: LeverageRiskMap,
): LeverageRiskEntry[] {
  return accounts.flatMap((account) => {
    const item = risk.get(account.accountId);
    return item ? [{ accountName: account.accountName, display: item.display }] : [];
  });
}

function riskEntriesForPlatform(
  platform: Tree["platforms"][number],
  risk: LeverageRiskMap,
): LeverageRiskEntry[] {
  return platform.identities.flatMap((identity) =>
    riskEntriesForAccounts(identity.accounts, risk).map((entry) => ({
      ...entry,
      identityName: identity.displayName,
    })),
  );
}

function riskDescriptionFor(
  level: "identity" | "platform",
  entries: LeverageRiskEntry[],
): string | undefined {
  return entries.length > 0 ? highLeverageDescription(level, entries) : undefined;
}

/** Native disclosure buttons keep each level operable without ARIA tree navigation. */
function TreeNode({
  name,
  actionName = name,
  value,
  percent,
  riskDescription,
  level,
  open,
  onToggle,
  children,
  actions,
}: {
  name: string;
  actionName?: string;
  value: string | null | undefined;
  percent?: string | null;
  riskDescription?: string;
  level: "platform" | "identity" | "account";
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
  actions?: ReactNode;
}) {
  const contentId = useId();
  const amount = formatAmount(value);
  const showPercent =
    (level === "platform" || level === "identity") && parseDecimal(percent)?.gt(0);
  const describedBy = riskDescription
    ? `${contentId}-value ${contentId}-risk`
    : `${contentId}-value`;

  return (
    <li className={styles.treeNode}>
      <div className={styles.treeNodeRow}>
        <button
          type="button"
          className={`${styles.treeNodeButton} ${level === "platform" ? styles.treePlatform : level === "identity" ? styles.treeIdentity : ""}`}
          aria-label={`${open ? "Collapse" : "Expand"} ${actionName}`}
          aria-describedby={describedBy}
          aria-expanded={open}
          aria-controls={open ? contentId : undefined}
          onClick={onToggle}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? shared.chevronOpen : shared.chevron}
          />
          <span className={styles.treeNodeText}>
            <span className={styles.treeNodeLeft}>
              <span className={styles.treeNodeName}>{name}</span>
              {riskDescription ? (
                <Badge tone="bad" title={riskDescription}>
                  <span className={styles.leverageRiskBadge} data-leverage-warning="true">
                    <TriangleAlert size={iconSize.xs} aria-hidden="true" />
                    High leverage
                  </span>
                </Badge>
              ) : null}
            </span>
            <span className={styles.treeNodeRight}>
              <span className={styles.treeNodeValue} id={`${contentId}-value`}>
                <NumericText>{amount === "—" ? amount : `$${amount}`}</NumericText>
              </span>
              {showPercent ? (
                <span className={styles.treeNodeBadge}>
                  <NumericText>{formatPercent(percent)}</NumericText>
                </span>
              ) : null}
            </span>
          </span>
          {riskDescription ? (
            <span id={`${contentId}-risk`} className={shared.srOnly}>
              {riskDescription}
            </span>
          ) : null}
        </button>
        {actions}
      </div>
      {open ? <div id={contentId}>{children}</div> : null}
    </li>
  );
}
