import type { ReactNode } from "react";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { accountOptions, useIdentities } from "@/features/data/useIdentities";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

export interface HoldingsFilterValues {
  platform: string;
  identityId: string;
  accountId: string;
  symbol: string;
}

/** Platform, Identity, Account, and symbol filters shared by the flat holdings lists. */
export function HoldingsFilters({
  portfolioId,
  values,
  onChange,
  children,
  hideAccount = false,
}: {
  portfolioId: string;
  values: HoldingsFilterValues;
  onChange: (next: Partial<HoldingsFilterValues>) => void;
  children?: ReactNode;
  hideAccount?: boolean;
}) {
  const platforms = usePlatforms();
  const identities = useIdentities(portfolioId);
  const items = identities.data?.items ?? [];
  const platformIds = [...new Set(items.map((item) => item.platform))].sort();
  const visibleIdentities = items.filter(
    (item) => item.status !== "archived" && (!values.platform || item.platform === values.platform),
  );
  const accounts = accountOptions(visibleIdentities).filter(
    (account) => !values.identityId || account.identityId === values.identityId,
  );

  return (
    <form className={styles.filters} onSubmit={(event) => event.preventDefault()}>
      <label className={styles.field}>
        <span>Platform</span>
        <Select
          value={values.platform}
          onChange={(event) =>
            onChange({ platform: event.target.value, identityId: "", accountId: "" })
          }
        >
          <option value="">All</option>
          {platformIds.map((id) => (
            <option key={id} value={id}>
              {platformLabel(platforms.data?.items, id)}
            </option>
          ))}
        </Select>
      </label>
      <label className={styles.field}>
        <span>Identity</span>
        <Select
          value={values.identityId}
          onChange={(event) => onChange({ identityId: event.target.value, accountId: "" })}
        >
          <option value="">All</option>
          {visibleIdentities.map((item) => (
            <option key={item.id} value={item.id}>
              {item.displayName}
            </option>
          ))}
        </Select>
      </label>
      {!hideAccount ? (
        <label className={styles.field}>
          <span>Account</span>
          <Select
            value={values.accountId}
            onChange={(event) => onChange({ accountId: event.target.value })}
          >
            <option value="">All</option>
            {accounts.map((item) => (
              <option key={item.id} value={item.id}>
                {item.label}
              </option>
            ))}
          </Select>
        </label>
      ) : null}
      <label className={styles.field}>
        <span>Symbol</span>
        <TextInput
          value={values.symbol}
          placeholder="BTC"
          onChange={(event) => onChange({ symbol: event.target.value })}
        />
      </label>
      {children}
    </form>
  );
}
