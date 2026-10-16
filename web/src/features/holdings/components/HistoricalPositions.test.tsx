import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AccountPositionList } from "@/api";
import { HistoricalPositions } from "./HistoricalPositions";

const state = vi.hoisted(() => ({
  data: undefined as AccountPositionList | undefined,
  isError: false,
  matches: true,
}));

vi.mock("@/shared/lib/usePortfolioQuery", () => ({
  usePortfolioQuery: () => ({ data: state.data, isError: state.isError }),
}));
vi.mock("@/shared/lib/useValuationRevision", () => ({
  useValuationRevision: () => state.matches,
}));

beforeEach(() => {
  state.isError = false;
  state.matches = true;
  state.data = {
    context: { portfolioId: "portfolio", valuationRevision: "cycle:1", timezone: "UTC" },
    account: {
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
    items: [],
    issues: [],
  };
});

function text() {
  return renderToStaticMarkup(
    <HistoricalPositions accountId="account" cycleId="cycle" valuationRevision="cycle:1" />,
  ).replace(/<[^>]*>/g, "");
}

describe("Historical position availability", () => {
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

  it.each(["pending", "failed", "revision mismatch"])(
    "does not describe %s reads as confirmed empty reports",
    (condition) => {
      if (condition === "revision mismatch") state.matches = false;
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
