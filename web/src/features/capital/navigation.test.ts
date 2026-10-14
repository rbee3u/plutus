import { describe, expect, it } from "vitest";
import { capitalTabFromSearch } from "./navigation";

describe("Capital navigation", () => {
  it("resolves supported tabs without changing search parameters", () => {
    for (const [input, expected] of [
      ["", "flows"],
      ["?tab=flows", "flows"],
      ["?tab=events", "events"],
      ["?tab=unknown", "flows"],
    ]) {
      expect(capitalTabFromSearch(input)).toBe(expected);
    }
  });
});
