---
status: accepted
---

# ADR 0021: Frontend feature layout, page stylesheets, and component seams

## Decision

`web/src` has three top-level areas: `app` (shell, routing, theme), `features`
(one directory per product area), and `shared` (API client, UI primitives,
libraries, icons, styles). Features are `auth`, `overview`, `holdings`,
`capital`, `history`, `manage`, and `settings`; `features/data` was renamed
`manage` to match its navigation group, and its URLs keep their `/data/...`
paths. Within a feature the sub-layout is `pages/`, `components/`, `dialogs/`,
`hooks/`, and `lib/` where the content exists, and the feature's stylesheet sits
at `features/<name>/styles.module.css`.

Components depend on `shared` and on their own feature. Cross-feature imports
are limited to embedded display components and the helpers a page already
reuses: the overview page embeds the holdings Tree and the history preset
helpers, and history embeds `holdings/components/HistoricalPositions`. Hooks
that more than one feature needs live in `shared/lib`: platform labels
(`usePlatforms`), identity options (`useIdentities`), and the valuation row kits
(`valuationRows`). The manual balance target type, queries, actions, and change
mapping are shared inside `holdings/dialogs/manual.tsx`.

The single `features/pages.module.css` is split by audience, preserving every
rule, selector list, media block, and relative order:

- `shared/styles/page.module.css` holds classes used by `shared/` or by two or
  more features: page frames, tables, forms, feedback, badges.
- Each feature's `styles.module.css` holds classes used by exactly one feature.
  Feature files import their module as `styles` and the shared module as
  `shared`.

Dead classes from retired designs (16) were deleted. One inert rule
(`.rangeForm > .field`, which cannot match the rendered DOM) was dropped.
`@keyframes spin` lives in the overview module, the only module that animates
with it. Two classes that only co-apply with shared classes in compound
selectors moved into the shared module with them: `.positionMarginTrigger` in
`.tableWrap td:has(> .positionMarginTrigger)` and `.reportDetails` in
`.reportDetails > .metaList`. CSS-module hashing is per file, so a compound
selector spanning modules would silently stop applying; every kept rule now
matches within one module.

Cascade order across modules follows bundle module order, not source order. The
split preserved the original order: computed styles on :8081 for the co-applied
`.form`/`.settingsCard` and `.panel`/`.settingsCard` elements are unchanged, and
same-build reruns of the walkthrough screenshots are byte-identical wherever
data is static. A future shared rule that must win or lose against a feature
rule is co-located with its counterpart rather than left to import order.

Oversized components were split at real seams, none above roughly 400 lines: the
account report's balance column kit (`BalancesTable`), the valuation point
detail (`PointDetail`), the custom range panel (`CustomPeriodPanel`), the
catalog tabs (`ListingsTab`, `InstrumentsTab`), identity actions
(`useIdentityActions`, `IdentityActions`), the identity dialogs (per-form files
over a shared dialog frame), the cycle notice (`CycleNotice`), the overview hero
chart (`HeroChart`), and the history value-change row kit
(`lib/valueChangeRows`). Cohesive files below the threshold were left alone.

## Reason and trade-offs

The flat `features/` directory and one shared stylesheet mixed ownership with
usage, so removing a page left styles and helpers no reviewer could attribute.
The three-area layout makes ownership structural: a feature directory can be
read, changed, or deleted as a unit, and `shared` states what is deliberately
common. The trade-off is that cross-feature reuse now requires a promotion
decision into `shared` instead of an import path, which is the point.

Per-feature stylesheets keep a feature's cascade reviewable in one file and stop
edits in one feature from churning a file every other feature imports. The
trade-off is that a class genuinely shared by two features must move to the
shared module and gets no automatic warning when it stops being shared.

Splitting only at real seams keeps the extracted pieces cohesive:
`AccountDetail` kept its report checks and feedback while the table kit moved
out, and pages below the size threshold stay whole until a second seam appears.
The trade-off is uneven file sizes, which is preferable to fragments that exist
only to hit a number.

Documenting the layout instead of enforcing it matches the backend decision in
[ADR 0020](./0020-architecture-layering.md): drift is caught by review, not by a
lint configuration that fights ordinary refactors.

[Architecture](../architecture.md) describes the current frontend mechanisms.
