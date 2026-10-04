import { ChevronRight } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import type { HoldingsTree as Tree } from "@/api/generated/types.gen";
import { AccountDetail } from "@/features/holdings/AccountDetail";
import { formatAmount } from "@/shared/lib/amount";
import { iconSize } from "@/shared/icons/iconSizes";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/features/pages.module.css";

export function HoldingsTree({ portfolioId, tree }: { portfolioId: string; tree: Tree }) {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const titleId = useId();

  function toggle(key: string) {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <section className={styles.panel} aria-labelledby={titleId}>
      <h2 id={titleId}>Holdings tree</h2>
      <p className={styles.muted}>
        Expand an Identity to see its Accounts, then an Account for its Balances and positions.
      </p>
      {tree.platforms.length === 0 ? (
        <p className={styles.muted}>
          No accounts inside the boundary yet. Add an Identity, select its account types, enable the
          connection, and run a Collection Cycle.
        </p>
      ) : (
        <ul
          className={styles.holdingsTree}
          aria-label="Holdings tree at the latest Valuation Point"
        >
          {tree.platforms.map((platform) => (
            <TreeNode
              key={platform.platform}
              name={platform.label}
              value={platform.valueUsd}
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
                    level="identity"
                    open={expanded.has(`identity:${identity.id}`)}
                    onToggle={() => toggle(`identity:${identity.id}`)}
                  >
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
          ))}
        </ul>
      )}
    </section>
  );
}

/** Native disclosure buttons keep each level operable without ARIA tree navigation. */
function TreeNode({
  name,
  actionName = name,
  value,
  level,
  open,
  onToggle,
  children,
}: {
  name: string;
  actionName?: string;
  value: string | null | undefined;
  level: "platform" | "identity" | "account";
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  const contentId = useId();
  const amount = formatAmount(value);
  return (
    <li className={styles.treeNode}>
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
          className={open ? styles.chevronOpen : styles.chevron}
        />
        <span className={styles.treeNodeText}>
          <span className={styles.treeNodeName}>{name}</span>
          <span className={styles.treeNodeValue} id={`${contentId}-value`}>
            <NumericText>{amount === "—" ? amount : `$${amount}`}</NumericText>
          </span>
        </span>
      </button>
      {open ? <div id={contentId}>{children}</div> : null}
    </li>
  );
}
