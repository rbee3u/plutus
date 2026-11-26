import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ReadContext } from "@/api";
import { HoldingsSummary, NotionalBreakdown } from "./HoldingsSummary";

const context: ReadContext = {
  portfolioId: "portfolio",
  timezone: "Asia/Shanghai",
  cycleId: "cycle",
  evaluatedAt: "2026-11-26T10:27:00Z",
};

const complete = {
  filteredTotalUsd: "103725.62",
  filteredTotalCompleteness: "complete" as const,
  filteredTotalFreshness: "fresh" as const,
};

describe("HoldingsSummary", () => {
  it("renders the notional split detail", () => {
    const html = renderToStaticMarkup(
      <HoldingsSummary
        context={context}
        label="Total notional"
        total={complete}
        detail={<NotionalBreakdown linearUsd="38775.62" inverseUsd="64950" />}
      />,
    );
    expect(html).toContain('data-breakdown="true"');
    expect(html).toContain("Total notional");
    expect(html).toContain("$103,725.62");
    expect(html).toContain('aria-label="Notional by contract type"');
    expect(html).toContain("Linear");
    expect(html).toContain("$38,775.62");
    expect(html).toContain("Inverse");
    expect(html).toContain("$64,950");
    expect(html).not.toContain("Incomplete");
  });

  it("keeps unknown and confirmed zero distinct", () => {
    const unknown = renderToStaticMarkup(<NotionalBreakdown linearUsd={null} inverseUsd="64950" />);
    expect(unknown).toContain("—");
    expect(unknown).toContain("Unknown");
    expect(unknown).not.toContain("$0");

    const zero = renderToStaticMarkup(<NotionalBreakdown linearUsd="0" inverseUsd="0" />);
    expect(zero).toContain("$0");
    expect(zero).not.toContain("Unknown");
  });

  it("retains completeness and freshness qualifiers", () => {
    const html = renderToStaticMarkup(
      <HoldingsSummary
        context={context}
        label="Total notional"
        total={{
          ...complete,
          filteredTotalCompleteness: "incomplete",
          filteredTotalFreshness: "stale",
        }}
        detail={<NotionalBreakdown linearUsd="200" inverseUsd="500" />}
      />,
    );
    expect(html).toContain("Incomplete");
    expect(html).toContain("Stale");
  });

  it("omits the split for other holdings tabs", () => {
    const html = renderToStaticMarkup(
      <HoldingsSummary context={context} label="Total balance" total={complete} />,
    );
    expect(html).not.toContain("Notional by contract type");
    expect(html).not.toContain('data-breakdown="true"');
  });
});
