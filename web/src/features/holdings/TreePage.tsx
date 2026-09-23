import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { Fragment, useState } from "react";
import { useParams } from "react-router";
import { getHoldingsTree } from "@/api/generated/sdk.gen";
import type { AccountValuation } from "@/api/generated/types.gen";
import { AccountDetail } from "@/features/holdings/AccountDetail";
import { formatAmount, formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { iconSize } from "@/shared/icons/iconSizes";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { NO_POINT } from "@/shared/lib/text";
import { CompletenessBadge, FreshnessBadge, StatusBadge } from "@/shared/ui/Badge";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { IconButton } from "@/shared/ui/IconButton";
import { isOwnControl } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const COLUMN_COUNT = 8;

function count(n: number, singular: string, plural: string): string {
  return `${String(n)} ${n === 1 ? singular : plural}`;
}

export function TreePage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const query = useQuery({
    queryKey: ["holdings-tree", userId, portfolioId],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await getHoldingsTree({ path: { portfolioId } }), redirect),
  });
  const tree = query.data;
  const timezone = tree?.context.timezone;

  function toggle(accountId: string) {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(accountId)) {
        next.delete(accountId);
      } else {
        next.add(accountId);
      }
      return next;
    });
  }

  return (
    <section className={styles.wide}>
      <p className={styles.lead}>
        Values at the latest Valuation Point. Expand an account for the balances and positions of
        its current Account Report.
      </p>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(tree)}
        busy={query.isFetching}
        fallback="Holdings could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      {query.isPending && !tree ? <p className={styles.muted}>Loading holdings.</p> : null}
      {tree && !tree.context.pointId ? <p className={styles.notice}>{NO_POINT}</p> : null}
      {tree?.context.pointId ? (
        <>
          <ContextSummary context={tree.context} />
          <p className={styles.totalRow}>
            <span>Total</span>
            <span className={styles.totalValue}>{formatUsd(tree.totalUsd)}</span>
            <CompletenessBadge value={tree.completeness} />
          </p>
          <div className={styles.tableWrap}>
            <table>
              <caption className={styles.srOnly}>
                Holdings tree at the latest Valuation Point
              </caption>
              <thead>
                <tr>
                  <th scope="col">Name</th>
                  <th scope="col" className={styles.numeric}>
                    Value (USD)
                  </th>
                  <th scope="col">Completeness</th>
                  <th scope="col">Freshness</th>
                  <th scope="col">Method</th>
                  <th scope="col">Status</th>
                  <th scope="col">Report observed</th>
                  <th scope="col">
                    <span className={styles.srOnly}>Details</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {tree.platforms.length === 0 ? (
                  <tr>
                    <td colSpan={COLUMN_COUNT}>
                      No accounts inside the boundary yet. Add an Identity, select its account
                      types, enable the connection, and run a Collection Cycle.
                    </td>
                  </tr>
                ) : null}
                {tree.platforms.map((platform) => (
                  <Fragment key={platform.platform}>
                    <tr className={styles.treePlatform}>
                      <td>
                        {platform.label}{" "}
                        <span className={styles.cellMeta}>
                          {count(platform.identities.length, "identity", "identities")}
                        </span>
                      </td>
                      <td className={styles.numeric}>{formatAmount(platform.valueUsd)}</td>
                      <td>
                        <CompletenessBadge value={platform.completeness} />
                      </td>
                      <td colSpan={COLUMN_COUNT - 3} />
                    </tr>
                    {platform.identities.map((identity) => (
                      <Fragment key={identity.id}>
                        <tr className={styles.treeIdentity}>
                          <td>
                            <span className={styles.cellStack}>
                              <span>
                                <strong>{identity.displayName}</strong>{" "}
                                <span className={styles.cellMeta}>
                                  {count(identity.accounts.length, "account", "accounts")}
                                </span>
                              </span>
                              <span className={styles.cellMeta}>
                                {identity.status === "archived" ? "archived · " : ""}
                                connection <StatusBadge value={identity.connectionStatus} />
                              </span>
                            </span>
                          </td>
                          <td className={styles.numeric}>{formatAmount(identity.valueUsd)}</td>
                          <td>
                            <CompletenessBadge value={identity.completeness} />
                          </td>
                          <td colSpan={COLUMN_COUNT - 3} />
                        </tr>
                        {identity.accounts.map((account) => (
                          <AccountRows
                            key={account.accountId}
                            portfolioId={portfolioId}
                            account={account}
                            timezone={timezone}
                            open={expanded.has(account.accountId)}
                            onToggle={() => toggle(account.accountId)}
                          />
                        ))}
                      </Fragment>
                    ))}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
          <IssuesPanel issues={tree.issues} empty="No issues at the latest Valuation Point." />
        </>
      ) : null}
    </section>
  );
}

function AccountRows({
  portfolioId,
  account,
  timezone,
  open,
  onToggle,
}: {
  portfolioId: string;
  account: AccountValuation;
  timezone?: string;
  open: boolean;
  onToggle: () => void;
}) {
  const detailId = `account-detail-${account.accountId}`;
  return (
    <>
      <tr
        className={`${styles.treeAccount} ${open ? styles.openRow : styles.clickRow}`}
        onClick={(event) => {
          if (!isOwnControl(event.target)) {
            onToggle();
          }
        }}
      >
        <td>
          <span className={styles.cellStack}>
            <span>{account.accountName}</span>
            {account.selected ? null : <span className={styles.cellMeta}>not selected</span>}
          </span>
        </td>
        <td className={styles.numeric}>{formatAmount(account.valueUsd)}</td>
        <td>
          <CompletenessBadge value={account.completeness} />
        </td>
        <td>
          <FreshnessBadge value={account.freshness} />
        </td>
        <td>{account.method}</td>
        <td>
          <span className={styles.cellStack}>
            <StatusBadge value={account.status} />
            {account.errorClass ? (
              <span className={styles.cellMeta}>{account.errorClass}</span>
            ) : null}
          </span>
        </td>
        <td>{formatInstant(account.reportObservedAt, timezone)}</td>
        <td>
          <IconButton
            label={
              open
                ? `Collapse balances and positions for ${account.accountName}`
                : `Expand balances and positions for ${account.accountName}`
            }
            aria-expanded={open}
            aria-controls={open ? detailId : undefined}
            onClick={onToggle}
          >
            <ChevronRight
              size={iconSize.sm}
              aria-hidden="true"
              className={open ? styles.chevronOpen : styles.chevron}
            />
          </IconButton>
        </td>
      </tr>
      {open ? (
        <tr className={styles.detailRow} id={detailId}>
          <td colSpan={COLUMN_COUNT}>
            <AccountDetail portfolioId={portfolioId} account={account} />
          </td>
        </tr>
      ) : null}
    </>
  );
}
