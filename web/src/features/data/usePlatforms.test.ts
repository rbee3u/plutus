import type { Platform } from "@/api/generated/types.gen";
import { describe, expect, it } from "vitest";
import { marginModeLabel, structureLabel } from "./usePlatforms";

const platforms: Platform[] = [
  {
    id: "okx",
    name: "OKX",
    adapterAvailable: true,
    accountStructures: [
      {
        id: "okx_unified",
        name: "Unified account",
        accountTypes: [{ id: "trading", name: "Trading" }],
        marginModes: [
          { id: "okx_futures", name: "Futures mode" },
          { id: "okx_pm", name: "Portfolio margin" },
        ],
      },
    ],
  },
  {
    id: "binance",
    name: "Binance",
    adapterAvailable: true,
    accountStructures: [
      {
        id: "binance_classic",
        name: "Classic Trading",
        accountTypes: [{ id: "spot", name: "Spot" }],
        marginModes: [],
      },
    ],
  },
];

describe("Identity structure labels", () => {
  it("names the Account Structure, falling back to its code", () => {
    expect(structureLabel(platforms, "okx", "okx_unified")).toBe("Unified account");
    expect(structureLabel(undefined, "okx", "okx_unified")).toBe("okx_unified");
  });

  it("shows the reported Margin Mode, or that it is unknown before validation", () => {
    expect(marginModeLabel(platforms, "okx", "okx_unified", "okx_futures")).toBe("Futures mode");
    expect(marginModeLabel(platforms, "okx", "okx_unified", null)).toBe(
      "margin mode unknown until validated",
    );
    expect(marginModeLabel(undefined, "okx", "okx_unified", "okx_pm")).toBe("okx_pm");
  });

  it("shows nothing for a structure without Margin Modes", () => {
    expect(marginModeLabel(platforms, "binance", "binance_classic", null)).toBeNull();
  });
});
