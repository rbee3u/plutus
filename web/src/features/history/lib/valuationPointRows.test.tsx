import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import {
  PERIOD_CLOSE_COLUMNS,
  RAW_POINT_COLUMNS,
  deltaChangeCell,
  periodCloseIssuesCell,
  rawPointIssuesCell,
  periodCloseRow,
  rawPointRow,
} from "./valuationPointRows";
import type { PeriodClose, ValuationPointSummary } from "@/api";

describe("valuationPointRows", () => {
  it.each([
    ["complete", "complete", []],
    ["incomplete", "complete", ["previous"]],
    ["complete", "incomplete", ["current"]],
    ["incomplete", "incomplete", ["previous", "current"]],
  ] as const)(
    "qualifies changes from %s to %s points in both tables",
    (before, after, affected) => {
      const previous: ValuationPointSummary = {
        cycleId: "before",
        valuationRevision: "before:1",
        calculationVersion: 1,
        detailState: "expired",
        evaluatedAt: "2026-10-11T10:58:00Z",
        totalUsd: "10439.46",
        completeness: before,
        accountCount: 10,
        staleAccountCount: 0,
        incompleteAccountCount: before === "incomplete" ? 8 : 0,
        boundaryContributionUsd: "0",
        boundaryWithdrawalUsd: "0",
      };
      const current: ValuationPointSummary = {
        ...previous,
        cycleId: "after",
        evaluatedAt: "2026-10-12T10:59:00Z",
        totalUsd: "71999.75",
        completeness: after,
        incompleteAccountCount: after === "incomplete" ? 8 : 0,
      };
      const close = (point: ValuationPointSummary): PeriodClose => ({
        periodKind: "day",
        label: point.evaluatedAt.slice(0, 10),
        periodStart: point.evaluatedAt,
        periodEnd: point.evaluatedAt,
        completeness: point.completeness,
        reasons: [],
        point,
      });
      const rows = [
        rawPointRow(current, previous, "UTC", "", () => {}),
        periodCloseRow(close(current), close(previous), "UTC"),
      ];
      for (const row of rows) {
        const html = renderToStaticMarkup(<>{row.change}</>);
        expect(html).toContain("+$61,560.29");
        expect(html).toContain("(+589.69%)");
        expect(html.includes("Incomplete")).toBe(affected.length > 0);
        for (const endpoint of ["previous", "current"] as const) {
          expect(html.includes(`The ${endpoint} Valuation Point is incomplete.`)).toBe(
            affected.some((value) => value === endpoint),
          );
        }
      }
      // A missing predecessor is not a zero or an incomplete numeric difference.
      const first = renderToStaticMarkup(
        <>{rawPointRow(current, undefined, "UTC", "", () => {}).change}</>,
      );
      expect(first).toContain("—");
      expect(first).not.toContain("Incomplete");
      const gap = { ...close(previous), point: undefined, completeness: "incomplete" as const };
      const missing = renderToStaticMarkup(
        <>{periodCloseRow(close(current), gap, "UTC").change}</>,
      );
      expect(missing).toContain("—");
      expect(missing).not.toContain("+$");
    },
  );

  it("defines 4 high-signal columns for Period Closes", () => {
    expect(PERIOD_CLOSE_COLUMNS.map((col) => col.header)).toEqual([
      "Period",
      "Close (USD)",
      "Change",
      "Issues",
    ]);
  });

  it("defines 5 high-signal columns for Raw Valuation Points", () => {
    expect(RAW_POINT_COLUMNS.map((col) => col.header)).toEqual([
      "Evaluated at",
      "Total (USD)",
      "Change",
      "Issues",
      "",
    ]);
  });

  it("formats deltaChangeCell with percentage and directional coloring", () => {
    const htmlPositive = renderToStaticMarkup(deltaChangeCell("132055.51", "131989.18"));
    expect(htmlPositive).toContain("+$66.33");
    expect(htmlPositive).toContain("(+0.05%)");

    const htmlNegative = renderToStaticMarkup(deltaChangeCell("131314.37", "131975.42"));
    expect(htmlNegative).toContain("-$661.05");
    expect(htmlNegative).toContain("(-0.50%)");

    const htmlZero = renderToStaticMarkup(deltaChangeCell("100.00", "100.00"));
    expect(htmlZero).toContain("$0.00 (0.00%)");

    const htmlBlank = renderToStaticMarkup(deltaChangeCell("100.00", null));
    expect(htmlBlank).toContain("—");
  });

  it("renders periodCloseIssuesCell quietly when healthy", () => {
    const item: PeriodClose = {
      periodKind: "day",
      label: "2026-10-09",
      periodStart: "2026-10-09T00:00:00Z",
      periodEnd: "2026-10-10T00:00:00Z",
      completeness: "complete",
      reasons: [],
    };
    const html = renderToStaticMarkup(periodCloseIssuesCell(item));
    expect(html).toContain("—");
  });

  it("renders periodCloseIssuesCell with warning badge when incomplete", () => {
    const item: PeriodClose = {
      periodKind: "day",
      label: "2026-10-09",
      periodStart: "2026-10-09T00:00:00Z",
      periodEnd: "2026-10-10T00:00:00Z",
      completeness: "incomplete",
      reasons: ["missing accounts"],
    };
    const html = renderToStaticMarkup(periodCloseIssuesCell(item));
    expect(html).toContain("Incomplete");
  });

  it("renders rawPointIssuesCell quietly when healthy", () => {
    const item: ValuationPointSummary = {
      cycleId: "pt1",
      valuationRevision: "pt1:1",
      calculationVersion: 1,
      detailState: "full" as const,
      evaluatedAt: "2026-10-09T14:12:00Z",
      totalUsd: "132055.51",
      completeness: "complete",
      accountCount: 36,
      staleAccountCount: 0,
      incompleteAccountCount: 0,
      boundaryContributionUsd: "0",
      boundaryWithdrawalUsd: "0",
    };
    const html = renderToStaticMarkup(rawPointIssuesCell(item));
    expect(html).toContain("—");
  });

  it("renders rawPointIssuesCell badges when stale or incomplete accounts exist", () => {
    const item: ValuationPointSummary = {
      cycleId: "pt2",
      valuationRevision: "pt2:1",
      calculationVersion: 1,
      detailState: "full" as const,
      evaluatedAt: "2026-10-09T14:12:00Z",
      totalUsd: "132055.51",
      completeness: "complete",
      accountCount: 36,
      staleAccountCount: 2,
      incompleteAccountCount: 1,
      boundaryContributionUsd: "0",
      boundaryWithdrawalUsd: "0",
    };
    const html = renderToStaticMarkup(rawPointIssuesCell(item));
    expect(html).toContain("2 stale");
    expect(html).toContain("1 incomplete");
  });

  it("constructs periodCloseRow with change from previous close", () => {
    const item: PeriodClose = {
      periodKind: "day",
      label: "2026-10-09",
      periodStart: "2026-10-09T00:00:00Z",
      periodEnd: "2026-10-10T00:00:00Z",
      point: {
        cycleId: "p2",
        valuationRevision: "p2:1",
        calculationVersion: 1,
        detailState: "full" as const,
        evaluatedAt: "2026-10-09T14:12:00Z",
        totalUsd: "132055.51",
        completeness: "complete",
        accountCount: 36,
        staleAccountCount: 0,
        incompleteAccountCount: 0,
        boundaryContributionUsd: "0",
        boundaryWithdrawalUsd: "0",
      },
      completeness: "complete",
      reasons: [],
    };
    const prev: PeriodClose = {
      periodKind: "day",
      label: "2026-10-08",
      periodStart: "2026-10-08T00:00:00Z",
      periodEnd: "2026-10-09T00:00:00Z",
      point: {
        cycleId: "p1",
        valuationRevision: "p1:1",
        calculationVersion: 1,
        detailState: "full" as const,
        evaluatedAt: "2026-10-08T23:37:00Z",
        totalUsd: "131558.05",
        completeness: "complete",
        accountCount: 36,
        staleAccountCount: 0,
        incompleteAccountCount: 0,
        boundaryContributionUsd: "0",
        boundaryWithdrawalUsd: "0",
      },
      completeness: "complete",
      reasons: [],
    };

    const row = periodCloseRow(item, prev, "UTC");
    expect(row.id).toBe("day-2026-10-09T00:00:00Z");
    expect(row.totalUsd).toBeDefined();
    expect(row.change).toBeDefined();
    expect(row.issues).toBeDefined();
  });

  it("constructs rawPointRow with change from previous point", () => {
    const item: ValuationPointSummary = {
      cycleId: "pt2",
      valuationRevision: "pt2:1",
      calculationVersion: 1,
      detailState: "full" as const,
      evaluatedAt: "2026-10-09T14:12:00Z",
      totalUsd: "132055.51",
      completeness: "complete",
      accountCount: 36,
      staleAccountCount: 0,
      incompleteAccountCount: 0,
      boundaryContributionUsd: "0",
      boundaryWithdrawalUsd: "0",
    };
    const prev: ValuationPointSummary = {
      cycleId: "pt1",
      valuationRevision: "pt1:1",
      calculationVersion: 1,
      detailState: "full" as const,
      evaluatedAt: "2026-10-09T12:17:00Z",
      totalUsd: "131989.18",
      completeness: "complete",
      accountCount: 36,
      staleAccountCount: 0,
      incompleteAccountCount: 0,
      boundaryContributionUsd: "0",
      boundaryWithdrawalUsd: "0",
    };

    const row = rawPointRow(item, prev, "UTC", "", () => {});
    expect(row.id).toBe("pt2");
    expect(row.totalUsd).toBeDefined();
    expect(row.change).toBeDefined();
    expect(row.issues).toBeDefined();
    expect(row.actions).toBeDefined();
  });
});
