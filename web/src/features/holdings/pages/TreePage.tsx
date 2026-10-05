import { ChevronRight, Plus } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import type { HoldingsTree as Tree } from "@/api";
import { type ManualTarget } from "../dialogs/manual";
import { ManualDialog, ManualPending } from "../dialogs/ManualBalances";
import { IconButton } from "@/shared/ui/IconButton";
import { AccountDetail } from "@/features/holdings/components/AccountDetail";
import { formatAmount, parseDecimal } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function HoldingsTree({ portfolioId, tree }: { portfolioId: string; tree: Tree }) {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [manualTarget, setManualTarget] = useState<ManualTarget | null>(null);
  const titleId = useId();
  const totalUsdDecimal = parseDecimal(tree.totalUsd);

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
      {tree.platforms.length === 0 ? (
        <p className={shared.muted}>
          No accounts inside the boundary yet. Add an Identity, select its account types, enable the
          connection, and run a Collection Cycle.
        </p>
      ) : (
        <ul
          className={styles.holdingsTree}
          aria-label="Holdings tree at the latest Valuation Point"
        >
          {tree.platforms.map((platform) => {
            const platformUsdDecimal = parseDecimal(platform.valueUsd);
            const percent =
              totalUsdDecimal && platformUsdDecimal && !totalUsdDecimal.isZero()
                ? platformUsdDecimal.dividedBy(totalUsdDecimal).times(100).toNumber()
                : 0;
            return (
              <TreeNode
                key={platform.platform}
                name={platform.label}
                value={platform.valueUsd}
                percent={percent}
                level="platform"
                open={!expanded.has(`platform:${platform.platform}`)}
                onToggle={() => toggle(`platform:${platform.platform}`)}
              >
                <ul className={styles.treeChildren}>
                  {platform.identities.map((identity) => (
                    <TreeNode
                      key={identity.id}
                      name={identity.displayName}
                      value={identity.valueUsd}
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
                      <ManualPending
                        portfolioId={portfolioId}
                        identityId={identity.id}
                        onOpen={setManualTarget}
                      />
                      <ul className={styles.treeChildren}>
                        {identity.accounts.map((account) => (
                          <TreeNode
                            key={account.accountId}
                            name={account.accountName}
                            actionName={`balances and positions for ${account.accountName}`}
                            value={account.valueUsd}
                            level="account"
                            open={expanded.has(`account:${account.accountId}`)}
                            onToggle={() => toggle(`account:${account.accountId}`)}
                          >
                            <div className={`${styles.treeChildren} ${styles.treeAccountDetail}`}>
                              <AccountDetail
                                portfolioId={portfolioId}
                                account={account}
                                onManual={setManualTarget}
                                pointId={tree.context.pointId ?? ""}
                                timezone={tree.context.timezone}
                              />
                            </div>
                          </TreeNode>
                        ))}
                      </ul>
                    </TreeNode>
                  ))}
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

/** Native disclosure buttons keep each level operable without ARIA tree navigation. */
function TreeNode({
  name,
  actionName = name,
  value,
  percent,
  level,
  open,
  onToggle,
  children,
  actions,
}: {
  name: string;
  actionName?: string;
  value: string | null | undefined;
  percent?: number;
  level: "platform" | "identity" | "account";
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
  actions?: ReactNode;
}) {
  const contentId = useId();
  const amount = formatAmount(value);
  const showPercent = level === "platform" && percent !== undefined && percent > 0;

  return (
    <li className={styles.treeNode}>
      <div className={styles.treeNodeRow}>
        <button
          type="button"
          className={`${styles.treeNodeButton} ${level === "platform" ? styles.treePlatform : level === "identity" ? styles.treeIdentity : ""}`}
          aria-label={`${open ? "Collapse" : "Expand"} ${actionName}`}
          aria-describedby={`${contentId}-value`}
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
            </span>
            <span className={styles.treeNodeRight}>
              <span className={styles.treeNodeValue} id={`${contentId}-value`}>
                <NumericText>{amount === "—" ? amount : `$${amount}`}</NumericText>
              </span>
              {showPercent ? (
                <span className={styles.treeNodeBadge}>{percent.toFixed(1)}%</span>
              ) : null}
            </span>
          </span>
        </button>
        {actions}
      </div>
      {open ? <div id={contentId}>{children}</div> : null}
    </li>
  );
}
