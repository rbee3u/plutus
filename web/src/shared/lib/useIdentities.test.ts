import { describe, expect, it } from "vitest";
import type { Account, Identity, Platform } from "@/api";
import { accountOptions, accountPathLabel } from "@/shared/lib/useIdentities";

type PlatformCatalog = Pick<Platform, "id" | "name" | "accountStructures">;

const portfolioId = "00000000-0000-7000-8000-000000000001";

const binance: PlatformCatalog = {
  id: "binance",
  name: "Binance",
  accountStructures: [
    {
      id: "binance_classic",
      name: "Classic trading",
      accountTypes: [
        { id: "spot", name: "Spot" },
        { id: "funding", name: "Funding" },
      ],
      marginModes: [],
    },
  ],
};

const okx: PlatformCatalog = {
  id: "okx",
  name: "OKX",
  accountStructures: [
    {
      id: "okx_unified",
      name: "Unified account",
      accountTypes: [{ id: "trading", name: "Trading" }],
      marginModes: [],
    },
  ],
};

function account(
  identityId: string,
  id: string,
  accountType: string,
  displayName: string,
): Account {
  return { id, identityId, accountType, displayName, selected: true };
}

function identity(options: {
  id: string;
  platform: string;
  displayName: string;
  accountStructure: string;
  accounts: Account[];
  status?: Identity["status"];
}): Identity {
  return {
    id: options.id,
    portfolioId,
    platform: options.platform,
    displayName: options.displayName,
    accountStructure: options.accountStructure,
    marginMode: null,
    externalRef: null,
    status: options.status ?? "enabled",
    connection: { status: "enabled", credentialsConfigured: true },
    accounts: options.accounts,
    version: 1,
    createdAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
  };
}

describe("account option labels", () => {
  it("prefixes every identity name with its platform", () => {
    expect(accountPathLabel("Binance", "main", "Spot")).toBe("Binance · main · Spot");
  });

  it("returns null when a path part is unknown", () => {
    expect(accountPathLabel("Binance", "main", null)).toBeNull();
    expect(accountPathLabel(undefined, "main", "Spot")).toBeNull();
    expect(accountPathLabel("Binance", "", "Spot")).toBeNull();
  });

  it("falls back to the platform identifier while the catalog is unavailable", () => {
    const options = accountOptions(
      [
        identity({
          id: "identity-binance",
          platform: "binance",
          displayName: "main",
          accountStructure: "binance_classic",
          accounts: [account("identity-binance", "account-spot", "spot", "Spot")],
        }),
      ],
      undefined,
    );
    expect(options.map((option) => option.label)).toEqual(["binance · main · Spot"]);
  });
});

describe("account option order", () => {
  it("orders platforms by the catalog and accounts by type order", () => {
    const options = accountOptions(
      [
        identity({
          id: "identity-okx",
          platform: "okx",
          displayName: "main",
          accountStructure: "okx_unified",
          accounts: [account("identity-okx", "account-trading", "trading", "Trading")],
        }),
        identity({
          id: "identity-binance",
          platform: "binance",
          displayName: "main",
          accountStructure: "binance_classic",
          accounts: [
            account("identity-binance", "account-funding", "funding", "Funding"),
            account("identity-binance", "account-spot", "spot", "Spot"),
          ],
        }),
      ],
      [binance, okx],
    );
    expect(options.map((option) => option.label)).toEqual([
      "Binance · main · Spot",
      "Binance · main · Funding",
      "OKX · main · Trading",
    ]);
  });

  it("orders identities by name ignoring case", () => {
    const options = accountOptions(
      [
        identity({
          id: "identity-zeta",
          platform: "binance",
          displayName: "zeta",
          accountStructure: "binance_classic",
          accounts: [account("identity-zeta", "account-zeta", "spot", "Spot")],
        }),
        identity({
          id: "identity-alpha",
          platform: "binance",
          displayName: "Alpha",
          accountStructure: "binance_classic",
          accounts: [account("identity-alpha", "account-alpha", "spot", "Spot")],
        }),
      ],
      [binance],
    );
    expect(options.map((option) => option.label)).toEqual([
      "Binance · Alpha · Spot",
      "Binance · zeta · Spot",
    ]);
  });

  it("places unknown platforms and unknown account types last", () => {
    const options = accountOptions(
      [
        identity({
          id: "identity-kraken",
          platform: "kraken",
          displayName: "main",
          accountStructure: "kraken_unified",
          accounts: [account("identity-kraken", "account-manual", "manual", "Manual")],
        }),
        identity({
          id: "identity-binance",
          platform: "binance",
          displayName: "main",
          accountStructure: "binance_classic",
          accounts: [
            account("identity-binance", "account-manual", "manual", "Manual"),
            account("identity-binance", "account-spot", "spot", "Spot"),
          ],
        }),
        identity({
          id: "identity-okx",
          platform: "okx",
          displayName: "main",
          accountStructure: "okx_unified",
          accounts: [account("identity-okx", "account-trading", "trading", "Trading")],
        }),
      ],
      [binance, okx],
    );
    expect(options.map((option) => option.label)).toEqual([
      "Binance · main · Spot",
      "Binance · main · Manual",
      "OKX · main · Trading",
      "kraken · main · Manual",
    ]);
  });

  it("skips archived identities", () => {
    const options = accountOptions(
      [
        identity({
          id: "identity-archived",
          platform: "binance",
          displayName: "main",
          accountStructure: "binance_classic",
          accounts: [account("identity-archived", "account-spot", "spot", "Spot")],
          status: "archived",
        }),
      ],
      [binance],
    );
    expect(options).toEqual([]);
  });
});
