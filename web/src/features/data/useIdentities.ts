import { listIdentities } from "@/api/generated/sdk.gen";
import type { Identity } from "@/api/generated/types.gen";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

export function identityEtag(item: Identity): string {
  return `"identity:${String(item.version)}"`;
}

/** Identities of the Portfolio with their embedded Connection and Accounts. */
export function useIdentities(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: ["identities", userId, portfolioId],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await listIdentities({ path: { portfolioId } }), redirect),
  });
}

export interface AccountOption {
  id: string;
  identityId: string;
  platform: string;
  label: string;
  selected: boolean;
}

/** Every Account of the non-archived Identities, labelled "identity · account". */
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
