import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { PositionRow } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import {
  POSITION_COLUMNS,
  contractDetailRows,
  positionMarginDescription,
  positionRow,
} from "./PositionsTable";

const position: PositionRow = {
  displayLeverage: "3",
  marginUnavailableCode: null,
  contractSize: null,
  entryPrice: null,
  liquidationPrice: null,
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
  settlementMargin: "0.002",
  reportedMargin: "100",
  marginAssetSymbol: "USD",
  marginBasis: "initial",
  marginSource: "reported",
  reportedLeverage: "3",
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

describe("contract details", () => {
  it("reuses the Catalog futures values without redundant labels", () => {
    const delivery: PositionRow = {
      ...position,
      platform: "binance",
      kind: "linear_delivery",
      baseAssetSymbol: "ETH",
      quoteAssetSymbol: "USDT",
      settlementAssetSymbol: "USDT",
      contractSize: "1",
      expiryAt: "2026-11-15T00:00:00Z",
    };
    const rows = contractDetailRows(delivery, "Asia/Shanghai");
    expect(rows.map((row) => row.label)).toEqual([
      "Base",
      "Quote",
      "Settle",
      "Expiry",
      "Contract size",
    ]);
    const html = renderToStaticMarkup(
      <>
        {rows.map((row) => (
          <span key={row.label}>{row.value}</span>
        ))}
      </>,
    );
    const text = html.replace(/<[^>]*>/g, "");
    expect(text).toContain("2026-11-15 08:00");
    expect(text).not.toContain("Asia/Shanghai");
    expect(text).toContain("1 ETH");
    expect(text).not.toContain("contract =");
  });
  it("keeps the Catalog columns without expiry rows for perpetual contracts", () => {
    const rows = contractDetailRows(position, "Asia/Shanghai");
    expect(rows.map((row) => row.label)).toEqual(["Base", "Quote", "Settle", "Contract size"]);
    const size = rows.find((row) => row.label === "Contract size");
    expect(renderToStaticMarkup(<>{size?.value}</>).replace(/<[^>]*>/g, "")).toBe("—");
  });
  it("exposes tiny contract sizes as ordinary decimals to screen readers", () => {
    const rows = contractDetailRows({
      ...position,
      kind: "linear_perpetual",
      contractSize: "0.000001",
    });
    const size = rows.find((row) => row.label === "Contract size");
    const html = renderToStaticMarkup(<>{size?.value}</>);
    expect(html).toContain('aria-hidden="true">0.0<sub');
    expect(html).toContain(">5</sub>1 BTC");
    expect(html).toContain(">0.000001 BTC</span>");
  });
});

describe("shared position cells", () => {
  it("uses the agreed inner columns, Both signs, settlement amounts, and timezone", () => {
    expect(POSITION_COLUMNS.map(({ key, header }) => [key, header])).toEqual([
      ["symbol", "Symbol"],
      ["side", "Side"],
      ["size", "Size"],
      ["entryPrice", "Entry price"],
      ["markPrice", "Mark price"],
      ["liquidationPrice", "Liq. price"],
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
      notionalUsd: "200",
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
    expect(html).toContain("1 issue");
  });
  it.each([
    ["cross", position, "0.002 BTC"],
    [
      "isolated",
      { ...position, marginSetting: "isolated" as const, marginBasis: "allocated" as const },
      "0.002 BTC",
    ],
    ["portfolio", { ...position, portfolioMargin: true }, "0.002 BTC"],
    ["unknown", { ...position, settlementMargin: null, portfolioMargin: true }, "—"],
  ])("shows only the margin amount in the closed %s cell", (_, item, expected) => {
    const cell = positionRow(item, "Asia/Shanghai").margin;
    const html = renderToStaticMarkup(<>{cell}</>);
    expect(html.replace(/<[^>]*>/g, "")).toBe(expected);
  });
});

describe("Margin mode and leverage", () => {
  it("uses the supplied display multiplier independently of the source leverage", () => {
    expect(positionMarginDescription(position)).toBe("3x cross");
    expect(
      positionMarginDescription({
        ...position,
        marginSetting: "isolated",
        portfolioMargin: true,
        reportedLeverage: "10",
        displayLeverage: "1.884999999999999999999999999999999",
      }),
    ).toBe("1.88x isolated");
    expect(
      positionMarginDescription({ ...position, marginSetting: "isolated", displayLeverage: "0" }),
    ).toBe("0x isolated");
  });
  it("preserves known modes when the multiplier is unavailable", () => {
    expect(
      positionMarginDescription({ ...position, marginSetting: "isolated", displayLeverage: null }),
    ).toBe("isolated");
    expect(positionMarginDescription({ ...position, displayLeverage: null })).toBe("cross");
    expect(positionMarginDescription({ ...position, portfolioMargin: true })).toBe(
      "portfolio margin",
    );
    expect(positionMarginDescription({ ...position, marginSetting: null })).toBe(
      "margin mode unavailable",
    );
  });
});
