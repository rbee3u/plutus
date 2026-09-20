export const PLATFORMS = [
  {
    id: "01a0bdfc-1fa4-7b00-b990-a610a553e10c",
    code: "binance",
    name: "Binance",
    modes: ["binance_pm"] as const,
  },
  {
    id: "01a0bdfc-1fa4-7b08-bc44-a2a59e4665b8",
    code: "okx",
    name: "OKX",
    modes: ["okx_mcm", "okx_pm"] as const,
  },
  {
    id: "01a0bdfc-1fa4-7b10-990c-04aa9d8ac659",
    code: "bybit",
    name: "Bybit",
    modes: ["bybit_uta_2_0"] as const,
  },
] as const;

export type AccountMode = (typeof PLATFORMS)[number]["modes"][number];

export function platformLabel(id: string): string {
  return PLATFORMS.find((item) => item.id === id)?.name ?? id;
}

export function modesForPlatform(platformId: string): readonly AccountMode[] {
  return PLATFORMS.find((item) => item.id === platformId)?.modes ?? [];
}
