import { describe, expect, it } from "vitest";
import {
  holdingsCandidates,
  holdingsScopeHasMissingReport,
  normalizeHoldingsSelection,
} from "./useHoldingsFilters";

const identities = [
  { id: "bn", platform: "binance", displayName: "Main", status: "enabled" as const },
  { id: "bn-sub", platform: "binance", displayName: "Sub", status: "enabled" as const },
  { id: "ok", platform: "okx", displayName: "Main", status: "enabled" as const },
  { id: "old", platform: "kraken", displayName: "Old", status: "archived" as const },
];
const all = { platform: "", identityId: "", symbol: "no-matches" };

describe("Holdings selection", () => {
  it("uses configured candidates regardless of search and omits archived platforms", () => {
    const candidates = holdingsCandidates(all, identities);
    expect([...candidates.platforms]).toEqual(["binance", "okx"]);
    expect(candidates.identities.map((item) => item.id)).toEqual(["bn", "bn-sub", "ok"]);
  });
  it("limits each dimension only by the other dimension", () => {
    const candidates = holdingsCandidates(
      { ...all, platform: "binance", identityId: "bn" },
      identities,
    );
    expect([...candidates.platforms]).toEqual(["binance"]);
    expect(candidates.identities.map((item) => item.id)).toEqual(["bn", "bn-sub"]);
  });
  it("preserves platform All when selecting an identity", () => {
    expect(normalizeHoldingsSelection({ ...all, identityId: "bn" }, identities)).toEqual({
      ...all,
      identityId: "bn",
    });
  });
  it("clears a conflicting platform and retains a valid identity", () => {
    expect(
      normalizeHoldingsSelection({ ...all, platform: "okx", identityId: "bn" }, identities),
    ).toEqual({ ...all, identityId: "bn" });
  });
  it("clears unknown or archived choices while retaining a compatible platform", () => {
    expect(
      normalizeHoldingsSelection({ ...all, platform: "binance", identityId: "old" }, identities),
    ).toEqual({ ...all, platform: "binance" });
    expect(
      normalizeHoldingsSelection({ ...all, platform: "kraken", identityId: "missing" }, identities),
    ).toEqual(all);
  });
});

describe("Missing reports in the Holdings scope", () => {
  const accounts = identities.map((identity) => ({
    ...identity,
    accounts: [{ id: `${identity.id}-account` }],
  }));
  const issues = [{ code: "missing", accountId: "ok-account" }];

  it("retains uncertainty across symbol searches without waiting for identity metadata", () => {
    expect(holdingsScopeHasMissingReport(issues, all, [])).toBe(true);
  });

  it("limits missing reports to the selected platform and identity", () => {
    expect(holdingsScopeHasMissingReport(issues, { ...all, platform: "binance" }, accounts)).toBe(
      false,
    );
    expect(holdingsScopeHasMissingReport(issues, { ...all, identityId: "bn" }, accounts)).toBe(
      false,
    );
    expect(holdingsScopeHasMissingReport(issues, { ...all, identityId: "ok" }, accounts)).toBe(
      true,
    );
    expect(holdingsScopeHasMissingReport(issues, { ...all, platform: "okx" }, accounts)).toBe(true);
  });

  it("does not confuse stale reports or missing prices with unknown position sets", () => {
    expect(
      holdingsScopeHasMissingReport(
        [
          { code: "stale", accountId: "ok-account" },
          { code: "no_price", accountId: "ok-account" },
        ],
        all,
        accounts,
      ),
    ).toBe(false);
  });
});
