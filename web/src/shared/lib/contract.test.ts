import { describe, expect, it } from "vitest";
import { formatContractSize, type ContractSizeFacts } from "./contract";

describe("contract size presentation", () => {
  it.each([
    [
      "binance linear without a recorded size",
      {
        platform: "binance",
        kind: "linear_perpetual",
        baseAssetSymbol: "ETH",
        quoteAssetSymbol: "USDT",
        contractSize: null,
      },
      "1 ETH",
    ],
    [
      "digit-leading base symbol",
      {
        platform: "binance",
        kind: "linear_delivery",
        baseAssetSymbol: "1000PEPE",
        quoteAssetSymbol: "USDT",
        contractSize: "5",
      },
      "5 × 1000PEPE",
    ],
    [
      "inverse quote face amount",
      {
        platform: "binance",
        kind: "inverse_perpetual",
        baseAssetSymbol: "BTC",
        quoteAssetSymbol: "USD",
        contractSize: "100",
      },
      "100 USD",
    ],
    [
      "recorded size on another platform",
      {
        platform: "okx",
        kind: "linear_delivery",
        baseAssetSymbol: "ETH",
        quoteAssetSymbol: "USDT",
        contractSize: "0.01",
      },
      "0.01 ETH",
    ],
    [
      "unknown size outside binance linear",
      {
        platform: "kraken",
        kind: "linear_perpetual",
        baseAssetSymbol: "BTC",
        quoteAssetSymbol: "USD",
        contractSize: null,
      },
      "—",
    ],
  ] satisfies Array<[string, ContractSizeFacts, string]>)("formats %s", (_name, item, expected) => {
    expect(formatContractSize(item)).toBe(expected);
  });
});
