import { ChevronRight } from "lucide-react";
import { useId, useMemo, useState, type ReactNode } from "react";
import type { AccountValuation, Platform } from "@/api";
import { formatUsd } from "@/shared/lib/amount";
import {
  accountValuationBadges,
  groupAccountValuationsByPlatform,
  type AccountBadge,
  type AccountValuationGroup,
} from "@/shared/lib/valuationRows";
import { Badge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import { iconSize } from "@/shared/icons/iconSizes";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

/**
 * Platform → identity → account tree for one historical or cycle account list.
 * Platforms and identities start expanded, accounts start collapsed, and
 * multiple accounts may stay open. Account detail content is read lazily.
 */
export function AccountValuationsTree({
  accounts,
  timezone,
  platforms,
  title = "Accounts",
  note,
  empty = "No accounts were valued.",
  renderDetail,
}: {
  accounts: AccountValuation[];
  timezone?: string;
  platforms?: Platform[];
  title?: string;
  note?: ReactNode;
  empty?: ReactNode;
  renderDetail?: (account: AccountValuation) => ReactNode;
}) {
  const titleId = useId();
  const groups = useMemo(
    () => groupAccountValuationsByPlatform(accounts, platforms),
    [accounts, platforms],
  );
  const [closed, setClosed] = useState<Set<string>>(() => new Set());
  const [openAccounts, setOpenAccounts] = useState<Set<string>>(() => new Set());

  function toggleClosed(key: string) {
    setClosed((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function toggleAccount(key: string) {
    setOpenAccounts((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <section className={`${styles.treeCard} ${styles.treeCardCompact}`} aria-labelledby={titleId}>
      <div className={styles.treeCardHeader}>
        <h3 id={titleId} className={styles.treeCardTitle}>
          {title}
        </h3>
      </div>
      {note ? <p className={shared.muted}>{note}</p> : null}
      {groups.length === 0 ? (
        <p className={shared.muted}>{empty}</p>
      ) : (
        <ul className={styles.holdingsTree} aria-label={title}>
          {groups.map((platform) => {
            const platformKey = `platform:${platform.key}`;
            const platformOpen = !closed.has(platformKey);
            const platformBadges: AccountBadge[] = platform.incomplete
              ? [
                  {
                    key: "platform-incomplete",
                    tone: "warn",
                    label: "Incomplete",
                    title: "At least one account value is unknown; the subtotal is partial.",
                  },
                ]
              : [];
            return (
              <li key={platform.key} className={styles.treeNode}>
                <DisclosureRow
                  name={platform.label}
                  value={platform.totalUsd}
                  badges={platformBadges}
                  level="platform"
                  open={platformOpen}
                  onToggle={() => toggleClosed(platformKey)}
                >
                  <ul className={styles.treeChildren}>
                    {platform.identities.map((identity) => {
                      const identityKey = `identity:${identity.key}`;
                      const identityBadge = identityIssuesBadge(identity);
                      return (
                        <li key={identity.key} className={styles.treeNode}>
                          <DisclosureRow
                            name={identity.identityName}
                            value={identity.totalUsd}
                            badges={identityBadge ? [identityBadge] : []}
                            level="identity"
                            open={!closed.has(identityKey)}
                            onToggle={() => toggleClosed(identityKey)}
                          >
                            <ul className={styles.treeChildren}>
                              {identity.accounts.map((account) => (
                                <AccountRow
                                  key={account.accountId}
                                  account={account}
                                  timezone={timezone}
                                  open={openAccounts.has(account.accountId)}
                                  onToggle={() => toggleAccount(account.accountId)}
                                  renderDetail={renderDetail}
                                />
                              ))}
                            </ul>
                          </DisclosureRow>
                        </li>
                      );
                    })}
                  </ul>
                </DisclosureRow>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function identityIssuesBadge(identity: AccountValuationGroup): AccountBadge | null {
  if (identity.issuesCount === 0 && !identity.incomplete) {
    return null;
  }
  const parts: string[] = [];
  if (identity.issuesCount > 0) {
    parts.push(`${identity.issuesCount} ${identity.issuesCount === 1 ? "issue" : "issues"}`);
  }
  if (identity.incomplete) {
    parts.push("Incomplete");
  }
  return {
    key: "identity-issues",
    tone: "warn",
    label: parts.join(" · "),
    title: identity.incomplete
      ? "At least one account value is unknown; the subtotal is partial."
      : "One or more accounts in this identity have data issues.",
  };
}

function AccountRow({
  account,
  timezone,
  open,
  onToggle,
  renderDetail,
}: {
  account: AccountValuation;
  timezone?: string;
  open: boolean;
  onToggle: () => void;
  renderDetail?: (account: AccountValuation) => ReactNode;
}) {
  const badges = accountValuationBadges(account, timezone);
  if (!renderDetail) {
    return (
      <li className={styles.treeNode}>
        <div className={styles.treeNodeRow}>
          <div className={`${styles.treeNodeButton} ${styles.treeNodeStatic}`}>
            <span className={styles.treeNodeSpacer} aria-hidden="true" />
            <RowContent
              name={account.accountName}
              value={account.valueUsd}
              badges={badges}
              badgesInteractive
            />
          </div>
        </div>
      </li>
    );
  }
  return (
    <li className={styles.treeNode}>
      <DisclosureRow
        name={account.accountName}
        value={account.valueUsd}
        badges={badges}
        level="account"
        open={open}
        onToggle={onToggle}
        actionName={`balances and positions for ${account.accountName}`}
        contentClassName={`${styles.treeChildren} ${styles.treeAccountDetail} ${styles.treeAccountDetailCompact}`}
      >
        {renderDetail(account)}
      </DisclosureRow>
    </li>
  );
}

function DisclosureRow({
  name,
  value,
  badges,
  level,
  open,
  onToggle,
  actionName = name,
  contentClassName,
  children,
}: {
  name: string;
  value: string | null;
  badges: AccountBadge[];
  level: "platform" | "identity" | "account";
  open: boolean;
  onToggle: () => void;
  actionName?: string;
  contentClassName?: string;
  children: ReactNode;
}) {
  const contentId = useId();
  const descriptionId = useId();
  const levelClass =
    level === "platform"
      ? styles.treePlatform
      : level === "identity"
        ? styles.treeIdentity
        : undefined;
  return (
    <>
      <div className={styles.treeNodeRow}>
        <button
          type="button"
          className={`${styles.treeNodeButton}${levelClass ? ` ${levelClass}` : ""}`}
          aria-label={`${open ? "Collapse" : "Expand"} ${actionName}`}
          aria-describedby={descriptionId}
          aria-expanded={open}
          aria-controls={open ? contentId : undefined}
          onClick={onToggle}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? shared.chevronOpen : shared.chevron}
          />
          <RowContent name={name} value={value} badges={badges} />
          <span id={descriptionId} className={shared.srOnly}>
            {nodeDescription(value, badges)}
          </span>
        </button>
      </div>
      {open ? (
        <div id={contentId} className={contentClassName}>
          {children}
        </div>
      ) : null}
    </>
  );
}

function RowContent({
  name,
  value,
  badges,
  badgesInteractive = false,
}: {
  name: string;
  value: string | null;
  badges: AccountBadge[];
  /** Static rows keep the old hover/focus disclosures; rows inside a disclosure button cannot nest controls. */
  badgesInteractive?: boolean;
}) {
  return (
    <span className={styles.treeNodeText}>
      <span className={styles.treeNodeLeft}>
        <span className={styles.treeNodeName}>{name}</span>
        {badges.length > 0 ? (
          <span className={shared.statusGroup}>
            {badges.map((badge) =>
              badgesInteractive ? (
                <HoverDetails
                  key={badge.key}
                  label={<Badge tone={badge.tone}>{badge.label}</Badge>}
                  name={`${badge.label} for ${name}`}
                >
                  {badgeDetails(badge)}
                </HoverDetails>
              ) : (
                <Badge key={badge.key} tone={badge.tone} title={badge.title}>
                  {badge.label}
                </Badge>
              ),
            )}
          </span>
        ) : null}
      </span>
      <span className={styles.treeNodeRight}>
        <span className={styles.treeNodeValue}>
          <NumericText>{formatUsd(value)}</NumericText>
        </span>
      </span>
    </span>
  );
}

function badgeDetails(badge: AccountBadge): ReactNode {
  const lines = badge.title?.split("\n").filter(Boolean) ?? [];
  if (lines.length === 0) return <p>{badge.label}</p>;
  return lines.map((line, index) => <p key={index}>{line}</p>);
}

function nodeDescription(value: string | null, badges: AccountBadge[]): string {
  return [formatUsd(value), ...badges.map((badge) => badge.label)].join(". ");
}
