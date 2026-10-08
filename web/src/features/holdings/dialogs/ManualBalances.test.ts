import { describe, expect, it } from "vitest";
import type { ManualBalance, ManualBalanceState } from "@/api";
import { manualChanges } from "./manual";
const balance = (symbol: string, quantity: string): ManualBalance => ({
  symbol,
  quantity,
  listingId: symbol,
  assetSymbol: symbol,
  modifiedAt: "2026-10-05T00:00:00Z",
});
const state = (items: ManualBalance[], appliedItems: ManualBalance[]): ManualBalanceState => ({
  items,
  appliedItems,
  accountId: "manual",
  identityId: "identity",
  version: 3,
  appliedVersion: 1,
  pending: true,
});
describe("pending manual quantities", () => {
  it("separates additions, signed edits and deletions from collected quantities", () => {
    expect(
      manualChanges(
        state(
          [balance("BTC", "-2"), balance("USD", "100")],
          [balance("BTC", "1"), balance("USDT", "50")],
        ),
      ),
    ).toEqual([
      { symbol: "BTC", oldQuantity: "1", quantity: "-2" },
      { symbol: "USD", oldQuantity: "0", quantity: "100" },
      { symbol: "USDT", oldQuantity: "50", quantity: "0" },
    ]);
  });
  it("ignores equivalent decimal representations and never rounds a small liability", () => {
    expect(manualChanges(state([balance("BTC", "1.0")], [balance("BTC", "1")]))).toEqual([]);
    expect(manualChanges(state([balance("BTC", "-0.000000000000000000000000000001")], []))).toEqual(
      [{ symbol: "BTC", oldQuantity: "0", quantity: "-0.000000000000000000000000000001" }],
    );
  });
});
