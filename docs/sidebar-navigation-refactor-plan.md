# Sidebar navigation refactor plan

Status: implemented

Working commit sequence: [sidebar-navigation-refactor-checklist.md](./sidebar-navigation-refactor-checklist.md).

This document defines the accepted information architecture and implementation
sequence for simplifying the Plutus application navigation. It is an
implementation brief for contributors changing the React shell and related
routes. It does not change the domain model, JSON API, or Phase 4 integration
scope.

The domain terms in this plan follow the [glossary](../CONTEXT.md). The
implementation must continue to respect the distinctions established by ADRs
[0002](./adr/0002-separate-identities-accounts-assets-and-instruments.md),
[0006](./adr/0006-configure-identity-access-and-discover-catalog-entries.md),
[0007](./adr/0007-persist-normalized-observations-and-rebuild-derived-results.md),
and [0008](./adr/0008-support-user-owned-multiple-portfolios.md).

## 1. Why this refactor exists

The current sidebar exposes 16 working routes in five labeled groups plus an
ungrouped Overview. None of those routes is a placeholder. The problem is the
navigation model, not missing page behavior:

- the `Data` group mixes access configuration, catalog governance, normalized
  source facts, background work, and audit history;
- `Portfolio` appears as a group, a top-bar selector, and a Settings item;
- `Security` is nested under a Portfolio route even though passwords and
  Sessions belong to the authenticated User;
- closely related low-frequency pages consume separate first-level entries;
- the full list requires scrolling at the navigation density used by the
  current shell.

The accepted direction is observation-first. Frequently used Portfolio views
remain directly accessible. Lower-frequency management pages are consolidated
behind routed secondary navigation without deleting their capabilities or
their existing URLs.

## 2. Goals and non-goals

### Goals

- Reduce the module navigation from 16 entries to 9.
- Make daily Portfolio observation more prominent than configuration and
  diagnostics.
- Preserve direct access to Balances, Positions, and Exposure because they are
  distinct, high-value views.
- Consolidate related low-frequency pages with explicit routed tabs.
- Keep existing Portfolio-scoped child URLs working.
- Move User-owned security controls out of the Portfolio route hierarchy.
- Preserve the current responsive, collapsible, keyboard-accessible shell.
- Leave clear navigation space for later collection and Platform integration
  work without displaying unimplemented modules.

### Non-goals

- Do not change calculations, projections, source facts, or API contracts.
- Do not remove or reduce any existing page capability.
- Do not implement Platform adapters, collection scheduling, Identity refresh,
  or Connection validation.
- Do not add an Accounts page. Accounts remain automatically discovered and
  available as filters or references where already supported.
- Do not rename Value Change to profit, P&L, performance, or return.
- Do not merge Balance, Position, and Exposure into one domain concept.
- Do not physically combine existing page components merely because they share
  one sidebar entry.
- Do not add empty navigation entries for future work.

## 3. Accepted information architecture

The module navigation must use this exact order and these exact labels:

```text
Overview

PORTFOLIO
├─ Balances
├─ Positions
├─ Exposure
└─ History
   ├─ Valuations
   └─ Value Changes

MANAGE
├─ Capital
│  ├─ Baseline
│  └─ Flows
├─ Data Sources
│  ├─ Identities
│  └─ Connections
├─ Asset Catalog
│  ├─ Asset Listings
│  └─ Instruments
└─ Data Activity
   ├─ Observations
   ├─ Jobs
   └─ Audit Log
```

Only the nine top-level labels are sidebar links. The indented labels are
secondary views rendered within the corresponding destination.

### 3.1 Why Balances, Positions, and Exposure stay separate

These pages are related but not interchangeable:

- a Balance reports measures for one Asset in one Account;
- a Position is an open directional exposure in one Instrument and Account;
- Exposure is derived from Balances and Positions in comparable underlying
  Asset units while retaining long, short, and net amounts.

An umbrella such as `Holdings` would obscure those distinctions and add a click
to three frequent tasks. They therefore remain independent sidebar entries.

### 3.2 Why the other pages are consolidated

- History groups the two Portfolio-level derived time-series views. It must not
  be called `Performance` because Value Change is not an investment return.
- Capital groups the two manual source facts that determine Contributed
  Capital.
- Data Sources groups the Platform Identity being accessed with its configured
  Connection. The term `Accounts` must not be used for this entry.
- Asset Catalog remains independent because mapping and tracking Asset Listings
  and Instruments is a different task from configuring access.
- Data Activity groups low-frequency inspection, correction, job diagnosis,
  and change-tracing views. It does not redefine Observation, Job, or audit
  semantics.

## 4. Route and view contract

The consolidation is a navigation change, not a route deletion. Existing
Portfolio child URLs remain canonical for their individual views.

| Sidebar entry | Default destination                   | Routes that activate the entry                                                                 | Secondary views                     |
| ------------- | ------------------------------------- | ---------------------------------------------------------------------------------------------- | ----------------------------------- |
| Overview      | `/p/:portfolioId/overview`            | `/p/:portfolioId/overview`                                                                     | None                                |
| Balances      | `/p/:portfolioId/portfolio/balances`  | Same as destination                                                                            | None                                |
| Positions     | `/p/:portfolioId/portfolio/positions` | Same as destination                                                                            | None                                |
| Exposure      | `/p/:portfolioId/portfolio/exposure`  | Same as destination                                                                            | None                                |
| History       | `/p/:portfolioId/history/valuations`  | `/p/:portfolioId/history/valuations`, `/p/:portfolioId/history/value-changes`                  | `Valuations`, `Value Changes`       |
| Capital       | `/p/:portfolioId/capital/baseline`    | `/p/:portfolioId/capital/baseline`, `/p/:portfolioId/capital/flows`                            | `Baseline`, `Flows`                 |
| Data Sources  | `/p/:portfolioId/data/identities`     | `/p/:portfolioId/data/identities`, `/p/:portfolioId/data/connections`                          | `Identities`, `Connections`         |
| Asset Catalog | `/p/:portfolioId/data/catalog`        | Same as destination                                                                            | `Asset Listings`, `Instruments`     |
| Data Activity | `/p/:portfolioId/data/observations`   | `/p/:portfolioId/data/observations`, `/p/:portfolioId/data/jobs`, `/p/:portfolioId/data/audit` | `Observations`, `Jobs`, `Audit Log` |

Navigation metadata must model the default destination separately from the set
of routes that make an entry active. Active state must not be inferred only
from the link destination. For example, `History` remains active on both of its
child routes.

The existing page components may remain separate and be composed through React
Router layouts or a shared routed-tab shell. Browser back and forward actions,
bookmarks, refreshes, and direct deep links must continue to select the correct
secondary view.

Page headings must retain the specific domain meaning of the active child
view. A generic sidebar label must not replace headings such as `Portfolio
Valuation`, `Value Changes`, `Capital Baseline`, or `Capital Flows` where the
full term is needed.

## 5. Controls outside the module navigation

### 5.1 Portfolio controls

The top-bar Portfolio area must contain:

```text
Portfolio Switcher
New Portfolio
Portfolio Settings
```

`Portfolio Settings` moves from the sidebar to a clearly labeled action next
to, or within the control group of, the Portfolio switcher. Its existing route
remains canonical:

```text
/p/:portfolioId/settings/portfolio
```

The page continues to edit the selected Portfolio's name, timezone, and display
Asset and to expose deletion where permitted. Moving the entry must not change
its ownership or behavior.

### 5.2 User controls

Replace the standalone `Sign out` button with a User menu containing:

```text
Account & Security
Sign out
```

`Account & Security` must use this canonical User-scoped route:

```text
/account/security
```

The legacy route must redirect with replacement to the canonical route:

```text
/p/:portfolioId/settings/security  ->  /account/security
```

The account page may retain the authenticated application chrome, but its data
loading and mutations must not require a Portfolio identifier. Navigating to
and from the page must preserve the User's last active Portfolio context.

### 5.3 Sidebar utilities

The sidebar footer remains separate from module navigation and contains:

```text
Theme
Version label
Collapse / Expand
```

Keep Theme and the collapse control available in expanded and collapsed modes.
The version label is the placeholder product string `v0.1.0-dev`. It is visible
in Vite development and in the embedded production SPA, including the Agent
lane on port 8081. Do not hide it in production builds. Do not change the CLI
`plutus --version` string, which remains independently `dev`.

## 6. Interaction and accessibility requirements

- Use links for navigable views and buttons only for actions or menu toggles.
- Each consolidated entry must expose visible secondary navigation on every
  child route.
- The selected secondary view must be programmatically identifiable. Use
  appropriate tab semantics only if the implementation follows the expected
  keyboard behavior; otherwise use an accessible local navigation list.
- Preserve `aria-current` behavior for the active sidebar destination.
- The collapsed sidebar must provide an accessible name and tooltip for every
  icon-only destination.
- Opening a module from the mobile drawer must close the drawer and place the
  resulting page in the normal focus flow.
- Secondary navigation must remain usable without horizontal page overflow at
  supported mobile widths.
- Moving Portfolio and User controls must not remove their keyboard access,
  outside-click dismissal, Escape handling, or focus visibility.
- Existing last-good data, Retry, trace ID, issue, freshness, and completeness
  behavior on child pages must remain unchanged.

## 7. Staged implementation

The stages below are ordered. A later stage may be developed separately only
after its dependency contract is merged or otherwise shared unchanged.

### Stage 1 — Navigation model and regression coverage

- [x] Refactor the navigation metadata so one sidebar entry can own multiple
      active child routes while retaining one default destination.
- [x] Separate sidebar labels from child page-title metadata.
- [x] Keep item order, default destinations, active-route matching, and child
      page-title mappings in pure, directly reviewable helpers.
- [x] If a frontend test harness is introduced independently before this work,
      add focused coverage for those helpers. Do not add a new test framework
      solely for this navigation refactor.
- [x] Keep the rendered navigation behavior unchanged until the metadata and
      route-validation matrix are ready for the consolidated structure.

Exit condition: the navigation model can represent the target tree without
duplicating sidebar items or deriving page titles from only the default link.

### Stage 2 — Consolidated Portfolio and management destinations

- [x] Render the exact nine-entry navigation from section 3.
- [x] Add routed secondary navigation for History, Capital, Data Sources, and
      Data Activity.
- [x] Reuse the existing child routes and page components.
- [x] Keep Asset Catalog's `Asset Listings` and `Instruments` views, with
      `Asset Listings` as the default.
- [x] Verify that every child route activates exactly one sidebar entry.

Exit condition: all current page capabilities remain reachable, every existing
Portfolio child URL still loads directly, and only nine module entries appear
in the sidebar.

### Stage 3 — Correct settings ownership and placement

- [x] Move Portfolio Settings access to the Portfolio controls without changing
      `/p/:portfolioId/settings/portfolio`.
- [x] Add the User menu with `Account & Security` and `Sign out`.
- [x] Make `/account/security` the canonical security route.
- [x] Redirect the legacy Portfolio-scoped security route.
- [x] Ensure Account & Security does not require a Portfolio ID and preserves
      the last active Portfolio context.
- [x] Remove the empty `SETTINGS` sidebar group.

Exit condition: Portfolio-owned and User-owned settings are reached from their
correct contexts, and no security URL implies Portfolio ownership.

### Stage 4 — Footer, responsive behavior, and cleanup

- [x] Keep Theme and Collapse / Expand in the sidebar footer.
- [x] Show the version label `v0.1.0-dev` in expanded and collapsed footer
      states, including the embedded production SPA.
- [x] Remove obsolete navigation metadata, imports, icons, and styles that are
      no longer referenced.
- [x] Verify expanded, collapsed, and mobile navigation states.
- [x] Verify keyboard and screen-reader behavior for menus and secondary
      navigation.

Exit condition: no obsolete entry remains, the footer shows `v0.1.0-dev`, and
the refactored navigation works at every supported shell width.

### Stage 5 — Final verification and documentation sync

- [x] Review the final diff for accidental page, terminology, API, or route
      changes.
- [x] Update README statements only if the implemented navigation makes an
      existing description inaccurate; do not describe this plan as complete
      before runtime verification passes.
- [x] Change this document's status to `implemented` only after all acceptance
      criteria and repository validation requirements pass.

Exit condition: implementation, tests, runtime behavior, and documentation all
describe the same navigation.

## 8. Acceptance checklist

### Information architecture

- [x] The sidebar contains exactly 9 module links in the specified order.
- [x] The only labeled groups are `PORTFOLIO` and `MANAGE`; Overview remains
      ungrouped above them.
- [x] There is no sidebar `SETTINGS` group.
- [x] Exact labels and capitalization match section 3.
- [x] No navigation item describes Value Change as performance, profit, P&L, or
      return.

### Routes and active state

- [x] Every route in section 4 remains directly reachable.
- [x] History, Capital, Data Sources, and Data Activity remain active across all
      of their child routes.
- [x] Direct navigation and browser refresh select the correct secondary view.
- [x] The legacy security route redirects to `/account/security`.
- [x] Portfolio Settings retains its Portfolio-scoped route.
- [x] Unknown application routes retain the existing safe fallback behavior.

### Functional regression

- [x] Balances, Positions, Exposure, Valuations, Value Changes, Capital,
      Identities, Connections, Catalog, Observations, Jobs, Audit, Portfolio
      Settings, and Account & Security retain their existing behavior.
- [x] Portfolio switching does not leak route or query state between
      Portfolios.
- [x] Sign out, password change, and Session revocation still work.
- [x] No frontend change requires an API or database migration.

### Responsive and accessible behavior

- [x] Expanded, collapsed, and mobile sidebars expose all nine destinations.
- [x] Active destinations and selected secondary views have accessible state.
- [x] Menus and secondary navigation work with keyboard-only input.
- [x] Focus indicators remain visible, and mobile navigation closes after a
      destination is selected.
- [x] The sidebar no longer requires scrolling at the reference desktop layout
      solely to reach a module entry.

## 9. Required validation

Follow the current repository workflow in [AGENTS.md](../AGENTS.md). Because
this refactor affects the embedded SPA, routing, and user interaction, static
checks alone are insufficient.

At minimum, an implementing change must:

1. pass `make format`, `make build`, `make test`, and `make lint` in order;
2. start the isolated Agent lane on port 8081;
3. exercise every sidebar destination, consolidated secondary view, moved
   settings entry, User menu action, legacy security redirect, Portfolio
   switch, mobile drawer, and collapsed-sidebar path as a real user;
4. confirm that direct child-route loads and browser back/forward behavior work;
5. stop every process started for verification and confirm the Agent lane is
   down; and
6. report what was exercised, observed, and not covered.

An implementation is not complete merely because the nine labels render. The
route ownership, active states, moved controls, responsive behavior, and child
page regressions are part of the accepted refactor.
