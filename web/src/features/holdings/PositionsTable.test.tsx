import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { PositionRow } from "@/api/generated/types.gen";
import { DataTable } from "@/shared/ui/DataTable";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";

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
  unrealizedPnl: "0.000123456789",
  marginMode: "cross",
  portfolioMargin: false,
  settleMargin: "0.002",
  margin: "100",
  marginCurrency: "USD",
  marginBasis: "initial",
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
      "Margin",
      "Unrealized PnL",
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
    expect(html).toContain("Portfolio");
    expect(html).toContain("1 issue");
    expect(html).toContain("2026-10-03 08:00");
  });
});
