import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ExposureRow } from "@/api";
import { ExposureBadges, ExposureTable } from "./ExposureTable";

function row(
  overrides: Partial<ExposureRow> & Pick<ExposureRow, "canonicalAssetSymbol">,
): ExposureRow {
  return {
    platformSymbols: [overrides.canonicalAssetSymbol],
    netQuantity: null,
    state: "complete",
    priceUsd: null,
    priceSource: null,
    priceSourceSymbol: null,
    priceSourcePlatform: null,
    valueUsd: null,
    accountCount: 0,
    unknownAccountCount: 0,
    freshness: "fresh",
    issues: [],
    ...overrides,
  };
}

const rows: ExposureRow[] = [
  row({
    canonicalAssetSymbol: "BTC",
    netQuantity: "0.48213",
    priceUsd: "68420",
    priceSource: "spot",
    priceSourceSymbol: "BTC",
    priceSourcePlatform: "binance",
    valueUsd: "32987.3346",
    accountCount: 3,
  }),
  row({
    canonicalAssetSymbol: "DOGE",
    netQuantity: "12000",
    priceUsd: "0.1421",
    priceSource: "spot",
    priceSourceSymbol: "DOGE",
    priceSourcePlatform: "okx",
    valueUsd: "1705.2",
    accountCount: 2,
    unknownAccountCount: 1,
    state: "partial",
    issues: [
      {
        code: "exposure_unknown_position",
        detail: "position size unavailable",
        accountId: "account",
        accountName: "Trading",
        symbol: "DOGE",
      },
    ],
  }),
  row({
    canonicalAssetSymbol: "AVAX",
    netQuantity: "0",
    priceUsd: "34.2",
    priceSource: "spot",
    valueUsd: "0",
    accountCount: 2,
  }),
  row({ canonicalAssetSymbol: "LINK", accountCount: 1, unknownAccountCount: 1, state: "unknown" }),
  row({
    canonicalAssetSymbol: "USDT",
    netQuantity: "1000",
    priceUsd: "1",
    priceSource: "spot",
    valueUsd: "1000",
    accountCount: 2,
    freshness: "stale",
  }),
];

function render() {
  return renderToStaticMarkup(
    <ExposureTable
      rows={rows}
      openKey={null}
      detailId="detail"
      onToggle={() => {}}
      renderDetail={() => null}
      empty="No exposure"
    />,
  );
}

describe("portfolio exposure table", () => {
  it("renders columns and state badges in the approved order", () => {
    const html = render();
    const headers = ["Asset", "Net exposure", "Price (USD)", "Value (USD)", "Accounts", "Issues"];
    const positions = headers.map((header) => html.indexOf(`>${header}<`));
    expect(positions.every((position) => position >= 0)).toBe(true);
    expect([...positions].sort((a, b) => a - b)).toEqual(positions);
    expect(html).toContain("Portfolio exposure by canonical asset");
  });

  it("distinguishes partial, flat, and unknown rows", () => {
    const html = render();
    expect(html).toContain("DOGE");
    expect(html).toContain("Partial");
    expect(html).toContain("1 unknown");
    expect(html).toContain("Flat");
    expect(html).toContain("Unknown");
    expect(html).toContain("0.48213");
    expect(html).toContain("$32,987.33");
    expect(html).toContain("$1,705.2");
    expect(html).toContain("$0");
  });

  it("keeps a stale row visible in Issues and does not use profit colors", () => {
    const html = render();
    expect(html).toContain("Stale");
    expect(html).toContain("1,000");
    expect(html).not.toContain("pnlNegative");
    expect(html).not.toContain("danger");
  });

  it("renders only state badges in the compact asset cell", () => {
    expect(renderToStaticMarkup(<ExposureBadges row={rows[0]} />)).not.toContain("Stale");
    expect(renderToStaticMarkup(<ExposureBadges row={rows[1]} />)).toContain("Partial");
    expect(renderToStaticMarkup(<ExposureBadges row={rows[2]} />)).toContain("Flat");
    expect(renderToStaticMarkup(<ExposureBadges row={rows[3]} />)).toContain("Unknown");
    expect(renderToStaticMarkup(<ExposureBadges row={rows[4]} showFreshness />)).toContain("Stale");
  });

  it("labels the reference price trigger for keyboard and pointer users", () => {
    const html = render();
    expect(html).toContain('aria-label="BTC reference price"');
    expect(html).toContain("68,420");
  });
});
