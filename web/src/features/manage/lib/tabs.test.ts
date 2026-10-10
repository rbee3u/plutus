import { describe, expect, it } from "vitest";
import { catalogTabFromSearch } from "./tabs";

describe("Catalog links", () => {
  it("resolves supported categories without changing filter input", () => {
    for (const [input, expected] of [
      ["", "assets"],
      ["?tab=assets", "assets"],
      ["?tab=futures", "futures"],
      ["?tab=unknown", "assets"],
    ]) {
      expect(catalogTabFromSearch(input)).toBe(expected);
    }
  });
});
