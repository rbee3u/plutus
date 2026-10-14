import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { PositionRow } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import { POSITION_COLUMNS, positionMarginDescription, positionRow } from "./PositionsTable";

const position: PositionRow = {
  accountId: "account",
  accountName: "Trading",
  accountType: "trading",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  positionKey: "BTC-USD-SWAP:net",
  futuresContractId: "futuresContract",
  futuresSymbol: "BTC-USD-SWAP",
  kind: "inverse_perpetual",
  side: "short",
  positionMode: "one_way",
  sourceQuantity: "2",
  sourceQuantityUnit: "contracts",
  size: "-200",
  sizeSymbol: "USD",
  baseAssetSymbol: "BTC",
  quoteAssetSymbol: "USD",
  settlementAssetSymbol: "BTC",
  markPrice: "50000",
  unrealizedPnl: "0.000123456789",
  marginSetting: "cross",
  portfolioMargin: false,
  settleMargin: "0.002",
  margin: "100",
  marginAssetSymbol: "USD",
  marginBasis: "initial",
  marginSource: "reported",
  leverage: "3",
  notionalUsd: "200",
  freshness: "fresh",
  reportObservedAt: "2026-10-03T00:00:00Z",
  issues: [],
};

function render(item: PositionRow) {
  return renderToStaticMarkup(
    <DataTable
      caption="Positions"
      columns={POSITION_COLUMNS}
      rows={[positionRow(item, "Asia/Shanghai")]}
    />,
  );
}

describe("shared position cells", () => {
  it("uses the agreed inner columns, Both signs, settlement amounts, and timezone", () => {
    expect(POSITION_COLUMNS.map(({ key, header }) => [key, header])).toEqual([
      ["symbol", "Symbol"],
      ["side", "Side"],
      ["size", "Size"],
      ["entryPrice", "Entry Price"],
      ["markPrice", "Mark Price"],
      ["liquidationPrice", "Liq. Price"],
      ["unrealizedPnl", "Unrealized PnL"],
      ["margin", "Margin"],
      ["notionalUsd", "Notional (USD)"],
      ["issues", "Issues"],
    ]);
    const row = positionRow(position, "Asia/Shanghai");
    for (const [key, expected] of Object.entries({
      side: "Both",
      size: "-200 USD",
      entryPrice: "—",
      markPrice: "50,000",
      liquidationPrice: "—",
      unrealizedPnl: "+0.000123457 BTC",
      margin: "0.002 BTC",
      notionalUsd: "200.00",
      issues: "—",
    })) {
      const html = renderToStaticMarkup(<>{row[key]}</>);
      expect(html.replace(/<[^>]*>/g, ""), key).toBe(expected);
    }
    // Keep one composition check; named cells own the values and their placement.
    expect(render(position)).toContain("-200 USD");
  });
  it("gives hedged sides distinct row keys and preserves stale and issue badges", () => {
    const short: PositionRow = {
      ...position,
      positionKey: "BTC-USD-SWAP:short",
      positionMode: "hedged",
      size: "200",
      freshness: "stale",
      portfolioMargin: true,
      issues: [
        { code: "stale", detail: "Previous report." },
        { code: "position_notional_unknown", detail: "Missing price." },
      ],
    };
    const long: PositionRow = {
      ...short,
      positionKey: "BTC-USD-SWAP:long",
      side: "long",
    };
    const shortRow = positionRow(short, "Asia/Shanghai");
    const longRow = positionRow(long, "Asia/Shanghai");
    expect(shortRow.id).not.toBe(longRow.id);
    expect(renderToStaticMarkup(<>{shortRow.side}</>)).toContain("Short");
    expect(renderToStaticMarkup(<>{longRow.side}</>)).toContain("Long");
    const html = renderToStaticMarkup(<>{shortRow.issues}</>);
    expect(html).toContain("Stale");
    expect(html).toContain("1 Issue");
  });
  it.each([
    ["cross", position, "0.002 BTC"],
    [
      "isolated",
      { ...position, marginSetting: "isolated" as const, marginBasis: "allocated" as const },
      "0.002 BTC",
    ],
    ["portfolio", { ...position, portfolioMargin: true }, "0.002 BTC"],
    ["unknown", { ...position, settleMargin: null, portfolioMargin: true }, "—"],
  ])("shows only the margin amount in the closed %s cell", (_, item, expected) => {
    const cell = positionRow(item, "Asia/Shanghai").margin;
    const html = renderToStaticMarkup(<>{cell}</>);
    expect(html.replace(/<[^>]*>/g, "")).toBe(expected);
  });
});

describe("Margin mode and leverage", () => {
  it("shows configured Cross leverage and only the Portfolio regime", () => {
    expect(positionMarginDescription(position)).toBe("3x cross");
    expect(positionMarginDescription({ ...position, portfolioMargin: true })).toBe(
      "portfolio margin",
    );
    expect(
      positionMarginDescription({ ...position, portfolioMargin: true, settleMargin: null }),
    ).toBe("portfolio margin");
  });

  it("uses actual isolated funds and source precision rather than configured leverage", () => {
    const isolated: PositionRow = {
      ...position,
      marginSetting: "isolated",
      marginBasis: "allocated",
      settlementAssetSymbol: "USDT",
      settleMargin: "1000",
      notionalUsd: "1880",
      leverage: "10",
      portfolioMargin: true,
    };
    expect(positionMarginDescription(isolated)).toBe("1.88x isolated");
    expect(
      positionMarginDescription({
        ...isolated,
        notionalUsd: "1884.999999999999999999999999999999",
      }),
    ).toBe("1.88x isolated");
    expect(positionMarginDescription({ ...isolated, notionalUsd: "0" })).toBe("0x isolated");
    for (const settlementAssetSymbol of ["USD", "USDT", "USDC"]) {
      expect(
        positionMarginDescription({ ...isolated, settlementAssetSymbol, markPrice: null }),
      ).toBe("1.88x isolated");
    }
  });

  it("converts Base-settled isolated margin to the same unit as USD notional", () => {
    const isolated: PositionRow = {
      ...position,
      marginSetting: "isolated",
      marginBasis: "allocated",
    };
    // 0.002 BTC at 50,000 USD/BTC is 100 USD of allocated margin.
    expect(positionMarginDescription(isolated)).toBe("2x isolated");
    expect(isolated.settleMargin).toBe("0.002");
    expect(isolated.leverage).toBe("3");
  });

  it("keeps the known mode when actual leverage cannot be established", () => {
    const isolated: PositionRow = {
      ...position,
      marginSetting: "isolated",
      marginBasis: "allocated",
    };
    for (const change of [
      { settleMargin: null },
      { settleMargin: "0" },
      { settleMargin: "-1" },
      { settleMargin: "NaN" },
      { notionalUsd: null },
      { notionalUsd: "Infinity" },
      { settlementAssetSymbol: null },
      { settlementAssetSymbol: "EUR" },
      { quoteAssetSymbol: "EUR" },
      { baseAssetSymbol: null },
      { markPrice: null },
      { markPrice: "0" },
      { markPrice: "-1" },
    ]) {
      expect(positionMarginDescription({ ...isolated, ...change })).toBe("isolated");
    }
    for (const leverage of [null, "0", "-1", "NaN"]) {
      expect(positionMarginDescription({ ...position, leverage })).toBe("cross");
    }
    expect(positionMarginDescription({ ...position, marginSetting: null })).toBe(
      "margin mode unavailable",
    );
  });
});
