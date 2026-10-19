import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "./valuationRows";
import type { BoundaryEvent, Platform } from "@/api";

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
