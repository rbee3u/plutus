import { describe, expect, it } from "vitest";
import { catalogTabFromSearch } from "./tabs";

describe("Catalog links", () => {
  it("resolves historical and current categories without changing filter input", () => {
    for (const [input, expected] of [
      ["", "assets"],
      ["?tab=listings&platform=okx&q=BTC&offset=50", "assets"],
      ["?tab=instruments&platform=binance&q=BTC&offset=50", "futures"],
      ["?tab=assets", "assets"],
      ["?tab=futures", "futures"],
      ["?tab=unknown", "assets"],
    ]) {
      expect(catalogTabFromSearch(input)).toBe(expected);
    }
  });
});
