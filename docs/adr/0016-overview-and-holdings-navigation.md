---
status: accepted
---

# ADR 0016: Overview Tree and flat Holdings navigation

## Decision

Overview presents its metric cards and recent-point chart followed by the full
Platform -> Identity -> Account Tree. The separate Platform and Identity group
tables are removed. The Tree uses the existing report metrics and holding
presentation, without repeating the Portfolio total, point context, or
page-level Issues. Named omissions remain visible in the page-level Issues.

The Tree is a nested disclosure list, without table headers or node counts. Each
node shows its name and adjacent USD valuation; unknown values remain an em
dash. The whole node row is a native expansion button with an accessible name
and expanded state. Indentation and quiet vertical guides express the ownership
hierarchy. Expanded Account contents stay inside their parent container; wide
detail tables scroll locally on narrow screens.

Normal connection labels and repeated ancestor health badges are removed.
Failed, stale, and incomplete states appear inside Account details, alongside
source observation time and named omissions. Page-level Issues stay visible
while nodes are collapsed. Known report measures and holding tables appear
directly; unavailable measures are omitted without inventing values. Method,
Report ID, and source time remain available in a trailing, initially collapsed
Report details disclosure.

Platforms start expanded, Identities start collapsed, and Account details start
collapsed. Each level has a keyboard-operable expansion control. Data updates
preserve expansion; entering Overview again or changing Portfolio resets it.
Balances always appear in Account details. Futures, Earn, and Dual Investment
sections are hidden only after a successful empty read. Loading, failed reads,
and stale holdings remain explicit rather than being treated as empty.

Holdings offers Balances, Futures, Earn, and Dual Investment as top-level views,
with Balances as its default. Options remains deferred without placeholder tabs
or changes to offered Accounts. Earn detail behavior follows ADR 0018. Each view
remembers its own Platform, Identity, search, and pagination during the
application session, scoped by User and Portfolio. Pending searches belong to
the outgoing view. Explicit URLs, reload, and browser history take precedence
over that memory. The new browser paths match the view names; old Positions
paths are not retained because the project has not been released.

Display refresh follows the native browser behavior accepted in ADR 0015.
Overview and Activity list polling is removed; there are no per-page reload
buttons. Running Collection progress, completion reads, and updates after User
mutations remain. Scheduled backend collection is unchanged. A reload reads
existing data and never requests a Collection Cycle.

The summary and Tree are displayed only for a consistent Valuation Point.
Expanded details must describe the visible Account Report. Failed or conflicting
reads offer recovery, retain compatible prior data, and name its age through the
existing point/report timestamps. Public JSON contracts and valuation rules are
unchanged.

## Reason and trade-offs

The ownership Tree answers Overview's central question of how much there is and
where. Adjacent names and values make each ownership node a compact unit. A
nested list keeps wide Account details visually attached to their owner. Moving
repeated status badges into details reduces noise; collapsed nodes require the
single page-level Issues section to explain affected totals. Flat Holdings views
make the distinct holding presentations directly reachable and remove nested
navigation. Independent filters avoid carrying a contract Symbol search into a
Dual Investment Pair search or Balance Asset search with a different meaning.

Native reload supplies one consistent action across pages, while manual
Collection retains feedback for work the User explicitly started. An open page
can lag behind scheduled collection until reload or another explicit data read;
its displayed point time remains the reference. Full reload resets Tree
expansion, unshown view memory, and unsaved inputs under ADR 0015.

The existing latest-only endpoints can race with collection. Frontend validation
rejects inconsistent reads instead of expanding the public API; a race can
require retry or page reload. Technical query and navigation mechanisms belong
in architecture documentation.
