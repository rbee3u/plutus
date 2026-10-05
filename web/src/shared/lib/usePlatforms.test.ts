import type { Platform } from "@/api";
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
          { id: "okx_future_mode", name: "Future OKX mode" },
        ],
      },
      {
        id: "okx_future_structure",
        name: "Future OKX structure",
        accountTypes: [],
        marginModes: [{ id: "okx_future_mode", name: "Future OKX mode" }],
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
  it.each([
    ["binance", "binance_classic", "Classic"],
    ["binance", "binance_pm", "Unified"],
    ["okx", "okx_unified", "Unified"],
    ["bybit", "bybit_uta_2_0", "Unified"],
    ["kraken", "kraken_separate", "Classic"],
    ["kraken", "kraken_unified", "Unified"],
  ])("displays %s / %s as %s before and after the catalog loads", (platform, structure, label) => {
    expect(structureLabel(undefined, platform, structure)).toBe(label);
    expect(structureLabel(platforms, platform, structure)).toBe(label);
  });

  it("retains catalog names or raw identifiers for unrecognized structures", () => {
    expect(structureLabel(platforms, "okx", "okx_future_structure")).toBe("Future OKX structure");
    expect(structureLabel(undefined, "okx", "okx_future_structure")).toBe("okx_future_structure");
    expect(structureLabel(platforms, "binance", "okx_unified")).toBe("okx_unified");
  });

  it.each([
    ["okx", "okx_unified", "okx_spot", "SPOT"],
    ["okx", "okx_unified", "okx_futures", "FUTURES"],
    ["okx", "okx_unified", "okx_mcm", "MCM"],
    ["okx", "okx_unified", "okx_pm", "PM"],
    ["bybit", "bybit_uta_2_0", "bybit_isolated", "ISO"],
    ["bybit", "bybit_uta_2_0", "bybit_cross", "MCM"],
    ["bybit", "bybit_uta_2_0", "bybit_pm", "PM"],
  ])(
    "displays %s / %s / %s as %s without needing the catalog",
    (platform, structure, mode, label) => {
      expect(marginModeLabel(undefined, platform, structure, mode)).toBe(label);
      expect(marginModeLabel(platforms, platform, structure, mode)).toBe(label);
    },
  );

  it.each([
    ["okx", "okx_unified"],
    ["bybit", "bybit_uta_2_0"],
  ])("shows UNKNOWN for an unobserved %s mode", (platform, structure) => {
    for (const mode of [null, undefined, ""]) {
      expect(marginModeLabel(undefined, platform, structure, mode)).toBe("UNKNOWN");
      expect(marginModeLabel(platforms, platform, structure, mode)).toBe("UNKNOWN");
    }
  });

  it.each([
    ["binance", "binance_classic"],
    ["binance", "binance_pm"],
    ["kraken", "kraken_separate"],
    ["kraken", "kraken_unified"],
  ])("omits the Margin Mode for %s / %s", (platform, structure) => {
    expect(marginModeLabel(undefined, platform, structure, null)).toBeNull();
    expect(marginModeLabel(platforms, platform, structure, null)).toBeNull();
  });

  it("retains catalog names or raw identifiers for unrecognized modes", () => {
    expect(marginModeLabel(platforms, "okx", "okx_unified", "okx_future_mode")).toBe(
      "Future OKX mode",
    );
    expect(marginModeLabel(undefined, "okx", "okx_unified", "okx_future_mode")).toBe(
      "okx_future_mode",
    );
    expect(marginModeLabel(platforms, "okx", "okx_unified", "bybit_cross")).toBe("bybit_cross");
    expect(marginModeLabel(platforms, "okx", "okx_future_structure", "okx_future_mode")).toBe(
      "Future OKX mode",
    );
    expect(marginModeLabel(platforms, "okx", "okx_future_structure", null)).toBe("UNKNOWN");
    expect(marginModeLabel(undefined, "future_platform", "future_structure", null)).toBeNull();
  });

  it.each(["constructor", "toString", "__proto__"])(
    "treats %s as an unrecognized identifier",
    (identifier) => {
      expect(structureLabel(undefined, "okx", identifier)).toBe(identifier);
      expect(structureLabel(undefined, identifier, "okx_unified")).toBe("okx_unified");
      expect(marginModeLabel(undefined, "okx", "okx_unified", identifier)).toBe(identifier);
      expect(marginModeLabel(undefined, "okx", identifier, "okx_pm")).toBe("okx_pm");
    },
  );
});
