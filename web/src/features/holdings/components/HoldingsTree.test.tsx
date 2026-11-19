import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AccountValuation, Overview } from "@/api";
import type { LeverageRiskMap } from "../lib/leverageRisk";
import { HoldingsTree } from "./HoldingsTree";

const state = vi.hoisted(() => ({
  risk: new Map() as LeverageRiskMap,
  isError: false,
}));

vi.mock("../hooks/useLeverageRisk", () => ({
  useLeverageRisk: () => state,
}));

const account: AccountValuation = {
  sourceReportId: "report",
  reportId: "report",
  accountId: "account",
  accountName: "Portfolio margin",
  accountType: "portfolio_margin",
  identityId: "identity",
  identityName: "main",
  platform: "binance",
  selected: true,
  status: "succeeded",
  method: "reported",
  freshness: "fresh",
  completeness: "complete",
  valueUsd: "100",
  reportObservedAt: "2026-10-03T00:00:00Z",
  reportedEquityUsd: "100",
  adjustedEquityUsd: "100",
  availableEquityUsd: "80",
  initialMarginRatio: "0.1",
  maintenanceMarginRatio: "0.01",
  marginRatio: "4.2",
  omitted: [],
};

const tree: Overview = {
  context: {
    portfolioId: "portfolio",
    timezone: "UTC",
    cycleId: "cycle",
    evaluatedAt: "2026-10-03T00:00:00Z",
  },
  point: {
    cycleId: "cycle",
    calculationVersion: 1,
    detailState: "full",
    evaluatedAt: "2026-10-03T00:00:00Z",
    totalUsd: "100",
    completeness: "complete",
    accountCount: 1,
    staleAccountCount: 0,
    incompleteAccountCount: 0,
    boundaryContributionUsd: "0",
    boundaryWithdrawalUsd: "0",
  },
  platforms: [
    {
      platform: "binance",
      label: "Binance",
      valueUsd: "100",
      completeness: "complete",
      identities: [
        {
          id: "identity",
          displayName: "main",
          status: "enabled",
          connectionStatus: "enabled",
          valueUsd: "100",
          completeness: "complete",
          accounts: [account],
        },
      ],
    },
  ],
  issues: [],
};

function markup() {
  return renderToStaticMarkup(<HoldingsTree portfolioId="portfolio" tree={tree} />);
}

beforeEach(() => {
  state.risk = new Map();
  state.isError = false;
});

describe("HoldingsTree leverage warning", () => {
  it("badges the platform and identity rows without expanding the account", () => {
    state.risk = new Map([["account", { value: "3.24", display: "3.24x" }]]);
    const html = markup();

    expect(html.match(/>High leverage</g)).toHaveLength(2);
    expect(html).toContain(
      'title="High leverage in main / Portfolio margin (3.24x). Add margin to reduce risk."',
    );
    expect(html).toContain(
      'title="High leverage in Portfolio margin (3.24x). Add margin to reduce risk."',
    );
    expect(html).toContain("aria-describedby=");
  });

  it("keeps the same tree when there is no warning", () => {
    const html = markup();
    expect(html).not.toContain(">High leverage<");
    expect(html).not.toContain("High leverage in");
    expect(html).toContain("Binance");
    expect(html).toContain("main");
  });

  it("shows the neutral status when the bulk read fails", () => {
    state.isError = true;
    const html = markup();
    expect(html).toContain("Leverage warnings could not be checked.");
    expect(html).toContain('role="status"');
    expect(html).not.toContain(">High leverage<");
  });
});
