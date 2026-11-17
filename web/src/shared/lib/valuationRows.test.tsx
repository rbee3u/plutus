import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import {
  accountValuationBadges,
  BOUNDARY_COLUMNS,
  boundaryEventRow,
  groupAccountValuations,
  groupAccountValuationsByPlatform,
} from "./valuationRows";
import type { AccountValuation, BoundaryEvent, Platform } from "@/api";

describe("BOUNDARY_COLUMNS and boundaryEventRow", () => {
  const platforms = [
    {
      id: "binance",
      name: "Binance",
    },
  ] as Platform[];

  it("defines the expected columns in correct sequence", () => {
    expect(BOUNDARY_COLUMNS.map((col) => col.key)).toEqual([
      "evaluatedAt",
      "platform",
      "identity",
      "account",
      "direction",
      "valueUsd",
      "issues",
    ]);
  });

  it("renders healthy boundary event with silent issues and formatted columns", () => {
    const event: BoundaryEvent = {
      cycleId: "c1",
      evaluatedAt: "2026-10-14T15:00:00Z",
      accountId: "a1",
      accountName: "Spot",
      identityId: "i1",
      identityName: "Main",
      platform: "binance",
      direction: "contribution",
      method: "report",
      completeness: "complete",
      valueUsd: "5000",
    };
    const row = boundaryEventRow(event, "UTC", platforms);
    expect(renderToStaticMarkup(<>{row.platform}</>)).toContain("Binance");
    expect(renderToStaticMarkup(<>{row.identity}</>)).toContain("Main");
    expect(renderToStaticMarkup(<>{row.account}</>)).toContain("Spot");
    expect(renderToStaticMarkup(<>{row.issues}</>)).toContain("—");
  });

  it("renders incomplete boundary event with Incomplete badge in issues column", () => {
    const event: BoundaryEvent = {
      cycleId: "c2",
      evaluatedAt: "2026-10-14T15:00:00Z",
      accountId: "a2",
      accountName: "Funding",
      identityId: null,
      identityName: null,
      platform: null,
      direction: "withdrawal",
      method: "report",
      completeness: "incomplete",
      valueUsd: null,
    };
    const row = boundaryEventRow(event, "UTC", platforms);
    expect(renderToStaticMarkup(<>{row.platform}</>)).toContain("—");
    expect(renderToStaticMarkup(<>{row.identity}</>)).toContain("—");
    expect(renderToStaticMarkup(<>{row.account}</>)).toContain("Funding");
    expect(renderToStaticMarkup(<>{row.valueUsd}</>)).toContain("—");
    expect(renderToStaticMarkup(<>{row.issues}</>)).toContain("Incomplete");
  });
});

describe("account valuation grouping", () => {
  const platforms = [
    { id: "binance", name: "Binance" },
    { id: "okx", name: "OKX" },
  ] as Platform[];

  const base: AccountValuation = {
    sourceReportId: null,
    reportId: "report-1",
    reportObservedAt: "2026-10-14T15:00:00Z",
    reportedEquityUsd: null,
    adjustedEquityUsd: null,
    availableEquityUsd: null,
    initialMarginRatio: null,
    maintenanceMarginRatio: null,
    marginRatio: null,
    accountId: "acc-1",
    accountName: "Spot",
    accountType: "spot",
    selected: true,
    identityId: "id-1",
    identityName: "Main",
    platform: "binance",
    status: "succeeded",
    method: "calculated",
    freshness: "fresh",
    completeness: "complete",
    valueUsd: "6268.18",
    omitted: [],
  };

  const account = (overrides: Partial<AccountValuation>): AccountValuation => ({
    ...base,
    ...overrides,
  });

  const margin = account({
    accountId: "acc-2",
    accountName: "Portfolio margin",
    accountType: "portfolio_margin",
    method: "reported",
    valueUsd: "6363.92",
  });

  const failed = account({
    accountId: "acc-3",
    accountName: "Spot",
    identityId: "id-2",
    identityName: "OKX main desk",
    platform: "okx",
    sourceReportId: null,
    reportObservedAt: null,
    status: "failed",
    method: "missing",
    freshness: "stale",
    completeness: "incomplete",
    valueUsd: null,
    errorClass: "API_UNAVAILABLE",
    errorDetail: "Network timeout",
  });

  it("groups by platform and identity while summing known values", () => {
    const groups = groupAccountValuationsByPlatform([base, margin, failed], platforms);

    expect(groups.map((group) => group.label)).toEqual(["Binance", "OKX"]);
    expect(groups[0].identities).toHaveLength(1);
    expect(groups[0].identities[0].identityName).toBe("Main");
    expect(groups[0].identities[0].totalUsd).toBe("12632.1");
    expect(groups[0].identities[0].incomplete).toBe(false);
    expect(groups[0].totalUsd).toBe("12632.1");
    expect(groups[0].incomplete).toBe(false);

    expect(groups[1].identities[0].issuesCount).toBe(1);
    expect(groups[1].identities[0].incomplete).toBe(true);
    expect(groups[1].identities[0].totalUsd).toBeNull();
    expect(groups[1].totalUsd).toBeNull();
    expect(groups[1].incomplete).toBe(true);
  });

  it("keeps a stale but complete subtotal complete", () => {
    const stale = account({ freshness: "stale", valueUsd: "100" });
    const [group] = groupAccountValuations([stale], platforms);

    expect(group.incomplete).toBe(false);
    expect(group.totalUsd).toBe("100");
    expect(group.issuesCount).toBe(1);
  });

  it("marks a subtotal incomplete when a known value is missing", () => {
    const missing = account({
      valueUsd: null,
      status: "failed",
      method: "missing",
      completeness: "incomplete",
    });
    const [group] = groupAccountValuations([base, missing], platforms);

    expect(group.incomplete).toBe(true);
    expect(group.totalUsd).toBe("6268.18");
  });

  it("describes collapsed account health with compact badges", () => {
    expect(accountValuationBadges(base)).toEqual([]);

    const failedBadges = accountValuationBadges(failed);
    expect(failedBadges.map((badge) => badge.label)).toEqual([
      "Failed",
      "Missing report",
      "Incomplete",
    ]);
    expect(failedBadges[0].tone).toBe("bad");

    const staleBadges = accountValuationBadges(account({ freshness: "stale" }), "UTC");
    expect(staleBadges.map((badge) => badge.label)).toEqual(["Stale"]);
    expect(staleBadges[0].title).toContain("Observed");

    const omittedBadges = accountValuationBadges(
      account({
        omitted: [{ assetSymbol: "ADA", equity: "12.5", reason: "unpriced" }],
      }),
    );
    expect(omittedBadges.map((badge) => badge.label)).toEqual(["1 omitted"]);
  });
});
