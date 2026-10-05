import { describe, expect, it } from "vitest";
import { holdingsCandidates, normalizeHoldingsSelection } from "./useHoldingsFilters";

const identities = [
  { id: "bn", platform: "binance", displayName: "Main", status: "enabled" as const },
  { id: "bn-sub", platform: "binance", displayName: "Sub", status: "enabled" as const },
  { id: "ok", platform: "okx", displayName: "Main", status: "enabled" as const },
  { id: "old", platform: "kraken", displayName: "Old", status: "archived" as const },
];
const all = { platform: "", identityId: "", symbol: "no-matches" };

describe("Holdings selection", () => {
  it("uses configured candidates regardless of search and omits archived Platforms", () => {
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
  it("preserves Platform All when selecting an Identity", () => {
    expect(normalizeHoldingsSelection({ ...all, identityId: "bn" }, identities)).toEqual({
      ...all,
      identityId: "bn",
    });
  });
  it("clears a conflicting Platform and retains a valid Identity", () => {
    expect(
      normalizeHoldingsSelection({ ...all, platform: "okx", identityId: "bn" }, identities),
    ).toEqual({ ...all, identityId: "bn" });
  });
  it("clears unknown or archived choices while retaining a compatible Platform", () => {
    expect(
      normalizeHoldingsSelection({ ...all, platform: "binance", identityId: "old" }, identities),
    ).toEqual({ ...all, platform: "binance" });
    expect(
      normalizeHoldingsSelection({ ...all, platform: "kraken", identityId: "missing" }, identities),
    ).toEqual(all);
  });
});
