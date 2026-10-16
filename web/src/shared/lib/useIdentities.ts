import { keys, listIdentities } from "@/api";
import type { Identity, IdentityList } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

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

/** Every account of the non-archived identities, labelled "identity · account". */
export function accountOptions(identities: Identity[] | undefined): AccountOption[] {
  return (identities ?? [])
    .filter((identity) => identity.status !== "archived")
    .flatMap((identity) =>
      identity.accounts.map((account) => ({
        id: account.id,
        identityId: identity.id,
        platform: identity.platform,
        label: `${identity.displayName} · ${account.displayName}`,
        selected: account.selected,
      })),
    );
}
