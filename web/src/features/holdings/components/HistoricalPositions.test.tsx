import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AccountReport } from "@/api";
import { HistoricalPositions } from "./HistoricalPositions";

const state = vi.hoisted(() => ({
  data: undefined as AccountReport | undefined,
  isError: false,
}));

vi.mock("@/shared/lib/usePortfolioQuery", () => ({
  usePortfolioQuery: () => ({ data: state.data, isError: state.isError }),
}));

beforeEach(() => {
  state.isError = false;
  state.data = {
    manualOverlay: null,
    context: {
      evaluatedAt: null,
      portfolioId: "portfolio",
      cycleId: "cycle",
      timezone: "UTC",
    },
    account: {
      sourceReportId: null,
      reportId: null,
      valueUsd: null,
      reportObservedAt: null,
      reportedEquityUsd: null,
      adjustedEquityUsd: null,
      availableEquityUsd: null,
      initialMarginRatio: null,
      maintenanceMarginRatio: null,
      marginRatio: null,
      accountId: "account",
      accountName: "Unified",
      accountType: "unified",
      identityId: "identity",
      identityName: "Owner",
      platform: "bybit",
      selected: true,
      status: "succeeded",
      method: "reported",
      freshness: "fresh",
      completeness: "complete",
      omitted: [],
    },
    positions: [],
    balances: [],
    earnPositions: [],
    dualPositions: [],
    issues: [],
  };
});

function text() {
  return renderToStaticMarkup(<HistoricalPositions accountId="account" cycleId="cycle" />).replace(
    /<[^>]*>/g,
    "",
  );
}

describe("Historical position availability", () => {
  it("retains position issues without attributing balance omissions to futures", () => {
    state.data!.issues = [
      { code: "stale", detail: "Earlier report retained." },
      { code: "position_notional_unknown", detail: "Position notional unavailable." },
      {
        code: "no_price",
        detail: "Balance price unavailable.",
        omittedBalance: { assetSymbol: "DEMO", equity: "2", reason: "no_price" },
      },
    ];
    expect(text()).toContain("Earlier report retained.");
    expect(text()).toContain("Position notional unavailable.");
    expect(text()).not.toContain("Balance price unavailable.");
  });
  it("keeps a missing successful source unknown even when a failed report has an ID", () => {
    Object.assign(state.data!.account, {
      reportId: "failed-report",
      method: "missing",
      status: "failed",
      freshness: "stale",
      completeness: "incomplete",
    });
    expect(text()).toContain("positions are unknown");
    expect(text()).not.toContain("No positions in this account report.");
  });

  it.each(["fresh", "stale"] as const)(
    "keeps a confirmed empty %s report distinct from an unknown position set",
    (freshness) => {
      Object.assign(state.data!.account, {
        freshness,
        status: freshness === "stale" ? "failed" : "succeeded",
      });
      expect(text()).toContain("No positions in this account report.");
      expect(text()).not.toContain("positions are unknown");
    },
  );

  it.each(["pending", "failed", "cycle mismatch"])(
    "does not describe %s reads as confirmed empty reports",
    (condition) => {
      if (condition === "cycle mismatch") state.data!.context.cycleId = "other-cycle";
      else state.data = undefined;
      state.isError = condition === "failed";
      expect(text()).toContain(
        state.isError
          ? "Historical positions could not be loaded."
          : "Loading historical positions.",
      );
      expect(text()).not.toContain("No positions in this account report.");
    },
  );
});
