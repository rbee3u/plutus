import { keys, listIdentities } from "@/api";
import type { Account, Identity, IdentityList, Platform } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { platformLabel } from "@/shared/lib/usePlatforms";

/** Identities of the portfolio with their embedded connection and accounts. */
export function useIdentities(portfolioId: string) {
  return usePortfolioQuery<IdentityList>({
    key: (userId, portfolioId) => keys.identities(userId, portfolioId),
    load: (portfolioId) => listIdentities({ path: { portfolioId } }),
    portfolioId,
  });
}

/** Call only after a successful identity read, independently of holdings. */
export function identitySetupGuidance(identities: Pick<Identity, "status">[]) {
  if (identities.some((identity) => identity.status !== "archived")) return null;
  return {
    title: identities.length === 0 ? "No identities yet" : "No active identities",
    help: "Add an identity to start tracking account values.",
  };
}

export interface AccountOption {
  id: string;
  identityId: string;
  platform: string;
  label: string;
  selected: boolean;
}

type PlatformCatalog = Pick<Platform, "id" | "name" | "accountStructures">[] | undefined;

/** `Platform · identity · account`, or null when any part is unknown. */
export function accountPathLabel(
  platform: string | null | undefined,
  identity: string | null | undefined,
  account: string | null | undefined,
): string | null {
  return platform && identity && account ? `${platform} · ${identity} · ${account}` : null;
}

function platformRank(platforms: PlatformCatalog, id: string): number {
  const index = platforms?.findIndex((item) => item.id === id) ?? -1;
  return index === -1 ? Number.MAX_SAFE_INTEGER : index;
}

function accountTypeRank(
  platforms: PlatformCatalog,
  platformId: string,
  structureId: string,
  accountType: string,
): number {
  const structure = platforms
    ?.find((item) => item.id === platformId)
    ?.accountStructures.find((item) => item.id === structureId);
  const index = structure?.accountTypes.findIndex((item) => item.id === accountType) ?? -1;
  return index === -1 ? Number.MAX_SAFE_INTEGER : index;
}

function compareText(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

/** Platform catalog order, identity name, then account type order. */
function compareAccounts(
  left: { identity: Identity; account: Account },
  right: { identity: Identity; account: Account },
  platforms: PlatformCatalog,
): number {
  return (
    platformRank(platforms, left.identity.platform) -
      platformRank(platforms, right.identity.platform) ||
    compareText(left.identity.platform, right.identity.platform) ||
    compareText(
      left.identity.displayName.toLowerCase(),
      right.identity.displayName.toLowerCase(),
    ) ||
    compareText(left.identity.id, right.identity.id) ||
    accountTypeRank(
      platforms,
      left.identity.platform,
      left.identity.accountStructure,
      left.account.accountType,
    ) -
      accountTypeRank(
        platforms,
        right.identity.platform,
        right.identity.accountStructure,
        right.account.accountType,
      ) ||
    compareText(left.account.id, right.account.id)
  );
}

/**
 * Every account of the non-archived identities, labelled `Platform · identity ·
 * account` so identities that share a name across platforms stay distinct.
 */
export function accountOptions(
  identities: Identity[] | undefined,
  platforms: PlatformCatalog,
): AccountOption[] {
  return (identities ?? [])
    .filter((identity) => identity.status !== "archived")
    .flatMap((identity) => identity.accounts.map((account) => ({ identity, account })))
    .sort((left, right) => compareAccounts(left, right, platforms))
    .map(({ identity, account }) => ({
      id: account.id,
      identityId: identity.id,
      platform: identity.platform,
      label:
        accountPathLabel(
          platformLabel(platforms, identity.platform),
          identity.displayName,
          account.displayName,
        ) ?? account.displayName,
      selected: account.selected,
    }));
}
