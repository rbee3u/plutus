import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import type { AccountValuation } from "@/api";
import { AccountDetail } from "./AccountDetail";

vi.mock("../hooks/useAccountReport", () => ({
  useAccountReport: () => {
    const query = { isSuccess: true, isPending: false, isError: false };
    return {
      balances: query,
      positions: query,
      dualPositions: query,
      earnPositions: query,
      isManual: false,
      isDual: false,
      isEarn: false,
    };
  },
}));

const account: AccountValuation = {
  accountId: "account",
  accountName: "Trading",
  accountType: "trading",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  selected: true,
  status: "succeeded",
  method: "reported",
  freshness: "fresh",
  completeness: "complete",
  marginRatio: "4.2",
  omitted: [],
};

function render(item: AccountValuation) {
  return renderToStaticMarkup(
    <AccountDetail
      portfolioId="portfolio"
      account={item}
      cycleId="cycle"
      valuationRevision="cycle:1"
    />,
  ).replace(/<[^>]*>/g, "");
}

describe("Account margin measures", () => {
  it.each([
    ["binance", "cross_margin", "Margin level", "999"],
    ["binance", "portfolio_margin", "UniMMR", "4.2"],
    ["okx", "trading", "Margin ratio", "4.2"],
  ])(
    "labels %s %s by its scope and leaves the source ratio unscaled",
    (platform, accountType, label, value) => {
      for (const freshness of ["fresh", "stale"] as const) {
        const text = render({
          ...account,
          platform,
          accountType,
          marginRatio: value,
          freshness,
          status: freshness === "stale" ? "failed" : "succeeded",
        });
        expect(text).toContain(`${label}${value}`);
        expect(text).not.toContain("%");
        for (const other of ["Margin level", "UniMMR", "Margin ratio"]) {
          if (other !== label) expect(text).not.toContain(other);
        }
      }
    },
  );

  it("omits unknown measures while preserving a reported zero", () => {
    expect(render({ ...account, marginRatio: null })).not.toContain("Margin ratio");
    expect(render({ ...account, marginRatio: "0" })).toContain("Margin ratio0");
  });
});
