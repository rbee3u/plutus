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
  it("uses configured candidates regardless of search and omits archived platforms", () => {
    const candidates = holdingsCandidates(all, identities);
    expect([...candidates.platforms]).toEqual(["binance", "okx"]);
    expect(candidates.identities).toEqual([]);
  });
  it("restricts identities to the selected platform in cascading mode", () => {
    const candidates = holdingsCandidates(
      { ...all, platform: "binance", identityId: "bn" },
      identities,
    );
    expect([...candidates.platforms]).toEqual(["binance", "okx"]);
    expect(candidates.identities.map((item) => item.id)).toEqual(["bn", "bn-sub"]);
  });
  it("clears identity when platform is All", () => {
    expect(normalizeHoldingsSelection({ ...all, identityId: "bn" }, identities)).toEqual({
      ...all,
      identityId: "",
    });
  });
  it("clears a conflicting identity when platform does not match", () => {
    expect(
      normalizeHoldingsSelection({ ...all, platform: "okx", identityId: "bn" }, identities),
    ).toEqual({ ...all, platform: "okx", identityId: "" });
  });
  it("clears unknown or archived choices while retaining a compatible platform", () => {
    expect(
      normalizeHoldingsSelection({ ...all, platform: "binance", identityId: "old" }, identities),
    ).toEqual({ ...all, platform: "binance", identityId: "" });
    expect(
      normalizeHoldingsSelection({ ...all, platform: "kraken", identityId: "missing" }, identities),
    ).toEqual(all);
  });
});
