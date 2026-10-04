---
status: accepted
---

# ADR 0015: Native mobile browser refresh

## Decision

The mobile layout supports the browser's native pull-to-refresh action in
ordinary iPhone Safari and Android Chrome tabs. The gesture reloads the current
URL and reads existing Plutus data. It does not request a Collection Cycle;
`Collect now` remains the explicit manual collection action.

This applies to all mobile pages, including sign-in and settings. Desktop users
use the browser reload button or shortcut. No page-specific refresh buttons are
added. Overview and Activity lists no longer poll periodically for display;
explicit action progress and post-action reads remain, as accepted in
[ADR 0016](./0016-overview-and-holdings-navigation.md). URL-backed filters and
pagination survive reload. Unsaved inputs and other transient page state follow
ordinary full-page reload behavior, without additional draft storage or refresh
interception.

The page title and navigation controls remain visible during scrolling. Open
navigation drawers and modal dialogs isolate background scrolling and restore
its previous position when the last overlay closes. Tables and overlay lists
retain their own appropriate scrolling.

## Reason and trade-offs

The accepted preference is the familiar browser refresh experience, rather than
an application-specific gesture or loading indicator. Native refresh requires
mobile document scrolling instead of the previous fixed-height inner pane.
Desktop retains its existing pane layout.

Full reload can discard unfinished edits, which is accepted here as the same
behavior as the browser refresh button. Browsers control the gesture threshold
and presentation. Embedded browsers and home-screen installations are outside
this delivery's acceptance scope.

Browser automation can verify scrolling, overlay lifecycle, and reload
semantics. Native browser chrome requires separate checks in actual Safari and
Android Chrome tabs; automated Chromium results do not establish that coverage.
