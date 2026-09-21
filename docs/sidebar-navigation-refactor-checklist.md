# Sidebar navigation refactor checklist

Working tracker for the staged implementation in
[sidebar-navigation-refactor-plan.md](./sidebar-navigation-refactor-plan.md).
Check an item in the same commit that finishes it. Phase 4 stays unchecked.

The contract, labels, and routes live in the plan. This file only sequences
commits and verification. Do not start a later stage until the previous stage
is merged or otherwise shared unchanged.

Validation follows [AGENTS.md](../AGENTS.md): after any SPA or routing change,
run `make format && make build && make test && make lint`, then
`./scripts/agent-wipe-and-serve.sh`, and exercise the stage exit condition on
`http://127.0.0.1:8081`. Stop every process started for that check before
moving on.

## Docs bootstrap

- [x] Record the footer version as always-visible `v0.1.0-dev` in the plan
      section 5.3, without changing CLI `plutus --version`
- [x] Add this checklist

## Stage 1 — Navigation model

- [ ] Separate sidebar labels, default destinations, and active path suffixes
- [ ] Map page titles independently of sidebar labels; keep current title
      strings
- [ ] Keep item order, default destinations, active-route matching, and titles
      in pure helpers
- [ ] Encode the target nine-module tree in reviewable constants without
      rendering it yet
- [ ] Keep the rendered 16-entry navigation unchanged

Exit: the model can represent the target tree; `:8081` still shows 16 working
entries with the current titles.

## Stage 2 — Nine modules and secondary navigation

- [ ] Render Overview plus `PORTFOLIO` and `MANAGE` with the nine module links
- [ ] Add routed secondary navigation (local `<nav>` links, not a tablist) for
      History, Capital, Data Sources, and Data Activity
- [ ] Reuse existing child routes and page components via layouts
- [ ] Convert Catalog to `?tab=` links; preserve other query filters; heading
      follows Asset Listings / Instruments
- [ ] Keep Portfolio Settings and Security in the sidebar until Stage 3
- [ ] Verify each child route activates exactly one module entry

Exit: all current capabilities remain reachable; only nine module entries plus
the temporary Settings group appear; child URLs still load directly.

## Stage 3 — Settings ownership

- [ ] Add `Portfolio Settings` next to `New Portfolio` in the switcher group
- [ ] Add the User menu (`username` trigger, Account & Security, Sign out)
- [ ] Make `/account/security` canonical; heading `Account & Security`
- [ ] Redirect `/p/:portfolioId/settings/security` with replacement and keep
      the last Portfolio id
- [ ] On `/account/security`, switching Portfolios stays on the page; creating
      a Portfolio opens its Overview
- [ ] Remove the empty `SETTINGS` sidebar group

Exit: Portfolio-owned and User-owned settings are reached from the correct
chrome, and no security URL implies Portfolio ownership.

## Stage 4 — Footer, responsive behavior, and cleanup

- [ ] Keep Theme and Collapse / Expand in the footer
- [ ] Show `v0.1.0-dev` in expanded and collapsed states
- [ ] Remove obsolete navigation metadata, imports, icons, and styles
- [ ] Verify expanded, collapsed, and mobile navigation

Exit: no obsolete entry remains, and navigation works at every supported shell
width.

## Stage 5 — Final verification and documentation

- [ ] Complete the plan section 8 acceptance matrix on `:8081`
- [ ] Update technical-design section 7.1 routes and shell chrome
- [ ] Update README only if an existing sentence became inaccurate
- [ ] Mark the plan implemented after verification passes

Exit: implementation, runtime behavior, and documentation describe the same
navigation.
