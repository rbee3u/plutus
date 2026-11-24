import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ExposureCoverage, ExposureSummary, ReadContext } from "@/api";
import { ExposureSummaryBar } from "./ExposureSummaryBar";

const context: ReadContext = {
  portfolioId: "portfolio",
  timezone: "Asia/Shanghai",
  cycleId: "cycle",
  evaluatedAt: "2026-11-24T05:01:00Z",
};

const coverage: ExposureCoverage = {
  complete: true,
  missingAccountIds: [],
  missingAccountCount: 0,
  staleAccountCount: 0,
  excludedAssetCount: 0,
  unattributedPositionCount: 0,
};

function summary(overrides: Partial<ExposureSummary> = {}): ExposureSummary {
  return {
    assetCount: 16,
    longCount: 15,
    shortCount: 1,
    flatCount: 0,
    partialCount: 0,
    unknownCount: 0,
    stablecoinValueUsd: "82260.40855",
    nonStablecoinValueUsd: "86020.10881061669",
    totalValueUsd: "168280.51736061668",
    unpricedAssetCount: 0,
    unpricedAssetSymbols: [],
    stablecoinSymbols: ["USD", "USDC", "USDT"],
    completeness: "complete",
    freshness: "fresh",
    ...overrides,
  };
}

function render(value: ExposureSummary, scope: ExposureCoverage = coverage, filtered = false) {
  return renderToStaticMarkup(
    <ExposureSummaryBar context={context} summary={value} coverage={scope} filtered={filtered} />,
  );
}

describe("ExposureSummaryBar", () => {
  it("shows the three signed USD amounts with secondary counts", () => {
    const html = render(summary());
    expect(html).toContain("PORTFOLIO EXPOSURE");
    expect(html).toContain("16 assets · 15 long · 1 short");
    expect(html).toContain("Stablecoins");
    expect(html).toContain("$82,260.41");
    expect(html).toContain("Non-stablecoins");
    expect(html).toContain("$86,020.11");
    expect(html).toContain("Total");
    expect(html).toContain("$168,280.52");
    expect(html).not.toContain("USD value is partial");
    expect(html).not.toContain("Incomplete");
  });

  it("marks a class with no known value as unknown and names the value omissions", () => {
    const html = render(
      summary({
        stablecoinValueUsd: null,
        totalValueUsd: "86020.10881061669",
        unpricedAssetCount: 2,
        unpricedAssetSymbols: ["USD", "USDT"],
        partialCount: 1,
        completeness: "incomplete",
      }),
      {
        ...coverage,
        complete: false,
        missingAccountCount: 1,
        missingAccountIds: ["account"],
      },
    );
    expect(html).toContain("—");
    expect(html).toContain("Unknown");
    expect(html).toContain("Incomplete");
    expect(html).toContain("1 partial");
    expect(html).toContain("USD value is partial");
    expect(html).toContain("2 assets without a USD value (USD, USDT)");
    expect(html).toContain("1 partial row");
    expect(html).toContain("1 account missing");
  });

  it("keeps known zero distinct from unknown and negative values signed", () => {
    const html = render(
      summary({
        stablecoinValueUsd: "-200",
        nonStablecoinValueUsd: "0",
        totalValueUsd: "-200",
      }),
    );
    expect(html).toContain("-$200");
    expect(html).toContain("$0");
    expect(html).not.toContain("Unknown");
  });

  it("labels a filtered summary", () => {
    expect(render(summary(), coverage, true)).toContain("FILTERED EXPOSURE");
    expect(render(summary(), coverage, true)).not.toContain("PORTFOLIO EXPOSURE");
  });
});
