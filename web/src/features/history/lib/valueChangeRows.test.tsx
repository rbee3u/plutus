import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import {
  VALUE_CHANGE_COLUMNS,
  valueChangeCell,
  combinedFlowsCell,
  valueChangeIssuesCell,
  valueChangeRow,
} from "./valueChangeRows";
import type { ValueChange } from "@/api";

describe("valueChangeRows", () => {
  it("defines 5 high-signal columns", () => {
    expect(VALUE_CHANGE_COLUMNS.map((col) => col.header)).toEqual([
      "Period",
      "Value change (USD)",
      "Close (USD)",
      "Flows",
      "Issues",
    ]);
  });

  it("formats valueChangeCell with percentage when startAmount is available", () => {
    const htmlPositive = renderToStaticMarkup(valueChangeCell("497.46", "131558.05"));
    expect(htmlPositive).toContain("+497.46");
    expect(htmlPositive).not.toContain("+$");
    expect(htmlPositive).toContain("(+0.3781%)");

    const htmlNegative = renderToStaticMarkup(valueChangeCell("-661.05", "131975.42"));
    expect(htmlNegative).toContain("-661.05");
    expect(htmlNegative).not.toContain("-$");
    expect(htmlNegative).toContain("(-0.5009%)");

    const htmlBlank = renderToStaticMarkup(valueChangeCell(null));
    expect(htmlBlank).toContain("—");
  });

  it("formats valueChangeCell with currency prefix when adorned option is enabled", () => {
    const htmlPositive = renderToStaticMarkup(
      valueChangeCell("497.46", "131558.05", { adorned: true }),
    );
    expect(htmlPositive).toContain("+$497.46");
    expect(htmlPositive).toContain("(+0.3781%)");

    const htmlNegative = renderToStaticMarkup(
      valueChangeCell("-661.05", "131975.42", { adorned: true }),
    );
    expect(htmlNegative).toContain("-$661.05");
    expect(htmlNegative).toContain("(-0.5009%)");
  });

  it("formats combinedFlowsCell silently when no flows exist", () => {
    const item: ValueChange = {
      amountUsd: null,
      periodKind: "day",
      label: "2026-10-09",
      from: "2026-10-09T00:00:00Z",
      to: "2026-10-10T00:00:00Z",
      comparable: true,
      flowsCompleteness: "complete",
      contributionsUsd: "0",
      withdrawalsUsd: "0",
      boundaryContributionsUsd: "0",
      boundaryWithdrawalsUsd: "0",
      completeness: "complete",
      issues: [],
      boundaryEvents: [],
    };
    const html = renderToStaticMarkup(combinedFlowsCell(item));
    expect(html).toContain("—");
  });

  it("formats combinedFlowsCell with flow types when flows exist", () => {
    const item: ValueChange = {
      amountUsd: null,
      periodKind: "day",
      label: "2026-10-09",
      from: "2026-10-09T00:00:00Z",
      to: "2026-10-10T00:00:00Z",
      comparable: true,
      flowsCompleteness: "complete",
      contributionsUsd: "1000",
      withdrawalsUsd: "0",
      boundaryContributionsUsd: "0",
      boundaryWithdrawalsUsd: "250",
      completeness: "complete",
      issues: [],
      boundaryEvents: [],
    };
    const html = renderToStaticMarkup(combinedFlowsCell(item));
    expect(html).toContain("+$1,000");
    expect(html).toContain("Capital flow");
    expect(html).toContain("-$250");
    expect(html).toContain("Boundary event");
  });

  it("renders quiet issues cell when healthy", () => {
    const item: ValueChange = {
      amountUsd: null,
      periodKind: "day",
      label: "2026-10-09",
      from: "2026-10-09T00:00:00Z",
      to: "2026-10-10T00:00:00Z",
      comparable: true,
      flowsCompleteness: "complete",
      contributionsUsd: "0",
      withdrawalsUsd: "0",
      boundaryContributionsUsd: "0",
      boundaryWithdrawalsUsd: "0",
      completeness: "complete",
      issues: [],
      boundaryEvents: [],
    };
    const html = renderToStaticMarkup(valueChangeIssuesCell(item));
    expect(html).toContain("—");
  });

  it("renders HoverDetails trigger badge when incomplete or has limitations", () => {
    const item: ValueChange = {
      amountUsd: null,
      periodKind: "day",
      label: "2026-10-08",
      from: "2026-10-08T00:00:00Z",
      to: "2026-10-09T00:00:00Z",
      comparable: true,
      flowsCompleteness: "complete",
      contributionsUsd: "0",
      withdrawalsUsd: "0",
      boundaryContributionsUsd: "0",
      boundaryWithdrawalsUsd: "0",
      completeness: "incomplete",
      issues: [{ code: "manual balance changed", detail: "manual balance changed" }],
      boundaryEvents: [],
    };
    const html = renderToStaticMarkup(valueChangeIssuesCell(item));
    expect(html).toContain("Incomplete");
  });

  it("constructs full row data properly", () => {
    const item: ValueChange = {
      periodKind: "day",
      label: "2026-10-09",
      from: "2026-10-09T00:00:00Z",
      to: "2026-10-10T00:00:00Z",
      startPoint: {
        cycleId: "p1",

        calculationVersion: 1,
        detailState: "full" as const,
        evaluatedAt: "2026-10-09T00:00:00Z",
        totalUsd: "100000",
        completeness: "complete",
        accountCount: 10,
        staleAccountCount: 0,
        incompleteAccountCount: 0,
        boundaryContributionUsd: "0",
        boundaryWithdrawalUsd: "0",
      },
      endPoint: {
        cycleId: "p2",

        calculationVersion: 1,
        detailState: "full" as const,
        evaluatedAt: "2026-10-10T00:00:00Z",
        totalUsd: "105000",
        completeness: "complete",
        accountCount: 10,
        staleAccountCount: 0,
        incompleteAccountCount: 0,
        boundaryContributionUsd: "0",
        boundaryWithdrawalUsd: "0",
      },
      amountUsd: "5000",
      comparable: true,
      flowsCompleteness: "complete",
      contributionsUsd: "0",
      withdrawalsUsd: "0",
      boundaryContributionsUsd: "0",
      boundaryWithdrawalsUsd: "0",
      completeness: "complete",
      issues: [],
      boundaryEvents: [],
    };
    const row = valueChangeRow(item, "UTC");
    expect(row.id).toBe("day-2026-10-09T00:00:00Z");
    expect(row.amountUsd).toBeDefined();
    const amountHtml = renderToStaticMarkup(row.amountUsd as React.ReactElement);
    expect(amountHtml).toContain("+5,000");
    expect(amountHtml).not.toContain("+$");
    expect(row.end).toBeDefined();
    expect(row.flows).toBeDefined();
    expect(row.issues).toBeDefined();
  });
});
