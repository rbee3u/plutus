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
  platformPositionRef: "BTC-USD-SWAP:net",
  instrumentId: "instrument",
  symbol: "BTC-USD-SWAP",
  kind: "inverse_perpetual",
  side: "short",
  positionMode: "one_way",
  quantity: "2",
  quantityUnit: "contracts",
  size: "-200",
  sizeSymbol: "USD",
  baseSymbol: "BTC",
  quoteSymbol: "USD",
  settlementSymbol: "BTC",
  markPrice: "50000",
  unrealizedPnl: "0.000123456789",
  marginMode: "cross",
  portfolioMargin: false,
  settleMargin: "0.002",
  margin: "100",
  marginCurrency: "USD",
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
    expect(POSITION_COLUMNS.map((item) => item.header)).toEqual([
      "Symbol",
      "Side",
      "Size",
      "Entry Price",
      "Mark Price",
      "Liq. Price",
      "Unrealized PnL",
      "Margin",
      "Notional (USD)",
      "Observed",
    ]);
    const html = render(position);
    expect(html).toContain("Both");
    expect(html).toContain("-200 USD");
    expect(html).toContain("0.000123457 BTC");
    expect(html).toContain("0.002 BTC");
    expect(html).toContain("2026-10-03 08:00");
    expect(html).not.toContain("Stale</span>");
    expect(html).not.toContain("Freshness");
    expect(html).not.toContain("Full amount");
  });
  it("keeps opposing hedged positions distinct and core badges visible", () => {
    const html = render({
      ...position,
      positionMode: "hedged",
      size: "200",
      freshness: "stale",
      portfolioMargin: true,
      issues: [
        { code: "stale", detail: "Previous report." },
        { code: "position_notional_unknown", detail: "Missing price." },
      ],
    });
    expect(html).toContain("Short");
    expect(html).toContain("Stale");
    expect(html).not.toContain(">pm</span>");
    expect(html).toContain("1 issue");
    expect(html).toContain("2026-10-03 08:00");
  });
  it("keeps margin mode and leverage out of the visible row", () => {
    for (const item of [
      position,
      { ...position, marginMode: "isolated" as const, marginBasis: "allocated" as const },
      { ...position, portfolioMargin: true },
      { ...position, settleMargin: null, portfolioMargin: true },
    ]) {
      const html = render(item);
      expect(html).not.toContain(">crx</span>");
      expect(html).not.toContain(">iso</span>");
      expect(html).not.toContain(">pm</span>");
      expect(html).not.toContain(">3x</span>");
      expect(html).not.toContain("positionMarginTagSlot");
    }
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
      marginMode: "isolated",
      marginBasis: "allocated",
      settlementSymbol: "USDT",
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
    for (const settlementSymbol of ["USD", "USDT", "USDC"]) {
      expect(positionMarginDescription({ ...isolated, settlementSymbol, markPrice: null })).toBe(
        "1.88x isolated",
      );
    }
  });

  it("converts Base-settled isolated margin to the same unit as USD notional", () => {
    const isolated: PositionRow = { ...position, marginMode: "isolated", marginBasis: "allocated" };
    // 0.002 BTC at 50,000 USD/BTC is 100 USD of allocated margin.
    expect(positionMarginDescription(isolated)).toBe("2x isolated");
    expect(isolated.settleMargin).toBe("0.002");
    expect(isolated.leverage).toBe("3");
  });

  it("keeps the known mode when actual leverage cannot be established", () => {
    const isolated: PositionRow = { ...position, marginMode: "isolated", marginBasis: "allocated" };
    for (const change of [
      { settleMargin: null },
      { settleMargin: "0" },
      { settleMargin: "-1" },
      { settleMargin: "NaN" },
      { notionalUsd: null },
      { notionalUsd: "Infinity" },
      { settlementSymbol: null },
      { settlementSymbol: "EUR" },
      { quoteSymbol: "EUR" },
      { baseSymbol: null },
      { markPrice: null },
      { markPrice: "0" },
      { markPrice: "-1" },
    ]) {
      expect(positionMarginDescription({ ...isolated, ...change })).toBe("isolated");
    }
    for (const leverage of [null, "0", "-1", "NaN"]) {
      expect(positionMarginDescription({ ...position, leverage })).toBe("cross");
    }
    expect(positionMarginDescription({ ...position, marginMode: null })).toBe(
      "margin mode unavailable",
    );
  });
});
