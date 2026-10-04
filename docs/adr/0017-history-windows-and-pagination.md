---
status: accepted
---

# ADR 0017: Explicit history windows and independent pagination

## Decision

Both History views default to the rolling last seven days. They expose 1d, 3d,
1w, 2w, 1Month, and 1Year shortcuts and populated From / To controls in the
Portfolio timezone. Days and weeks are fixed elapsed durations. Months and years
preserve local calendar time and clamp the day to the target month's last day.
Ambiguous or nonexistent manually entered wall times are rejected explicitly.

Shortcuts apply immediately. Manual edits apply together through Apply. A
relative window is anchored once for all reads during browsing; pagination,
detail expansion, and retries cannot move it. Reload resolves a relative window
again; custom absolute endpoints remain fixed. URLs retain the range, manually
selected period, and independent table pagination states.

The Portfolio value chart automatically uses Raw through three days, Hour
through 45 days, and Day for longer ranges. Hour and Day keep the final actual
point of each existing timezone bucket; they do not average or synthesize value.
Raw Valuation Points remain individually accessible in a separate paginated
table. Curves retain ascending time order; all History tables use newest first.

Valuation Points displays filters, the full-window chart, Period closes, then
raw points. Period closes shares the selected time window. Both calendar views
initially select Day for day/week shortcuts, Week for 1Month, and Month for
1Year. Users can override the period; applying a custom window retains it.
Calendar reads calculate complete intersecting periods, not clipped intervals.
Value Changes retains its independent Custom range comparison.

All three tables default to 20 rows with 10/20/50 choices, Previous / Next, and
row counts. Range changes reset pagination and open point details. Period
changes reset only their period table. Expanding a point cannot reset a page.

## Reason and trade-offs

Blank inputs hid an existing seven-day default, while hundreds of raw rows made
Period closes difficult to reach. Explicit windows and independent tables make
the query and page size visible without losing individual observations.

Raw points require database pagination because their count is unbounded.
Calendar results retain the existing 400-period ceiling and are paginated in the
browser, preserving the complete Value Changes chart without recomputing
financial results on each page change. Aggregated curve reads still scan narrow
point headers across the selected range; this is not stored preaggregation or a
retention policy. Query mechanisms belong in architecture documentation.

These changes do not change Period Close, Value Change, completeness, or the
Portfolio Boundary. Synthetic and agent checks do not establish exchange-app
validation or native mobile browser coverage.
