---
status: accepted
---

# ADR 0019: Manual balances and confirmed empty Accounts

## Decision

Every Identity owns one fixed, always-selected `manual` Account, displayed as
Manual. It supplements assets and liabilities missing from the Platform API. It
remains in the Portfolio Boundary when the Connection is disabled, invalid, or
unconfigured; archiving the Identity removes it from the boundary while
preserving records and history. External bank assets remain outside this scope.

One normalized Asset code has one current signed quantity. Duplicate creation
fails rather than accumulating. Zero deletes the current record. Quantity edits
and deletions require the ledger version, are audited, and append immutable
revisions. Changing an Asset code requires deletion and creation.

Saving affects the next Collection Cycle. The cycle persists its Manual inputs
before any Platform reads, and retries reuse those inputs. Manual reports are
immutable complete sets, independent of private collection outcomes. Public
market data uses the existing Platform implementations without credentials;
Manual valuation requires the Account's own Platform and the existing 24-hour
price limit. USD, USDT, and USDC retain their 1 USD rule. Missing prices
preserve signed quantities and explicit omissions. No reported equity,
availability, interest, or return is invented.

Quantity changes become effective at a completed Valuation Point. They produce
no Capital Flow; Value Change intervals crossing an effective adjustment retain
the numeric difference but become incomplete. No-op edits, price-only changes,
and edits reverted before collection do not mark adjustments. An existing
Identity receiving its first Manual report continues from zero without a false
boundary contribution. New Identities and archive retain ordinary signed
boundary treatment.

Overview offers an add action beside each Identity and Manual Balances heading.
Holdings Balances offers a global action, including before the first point.
These actions share an editor reading the current ledger, with edit and delete
controls restricted to Manual records. Pending additions, edits, and deletions
are separate from the immutable point values. Excluded records remain editable.
Users update or delete Manual quantities when settlement becomes API-visible;
the application does not infer settlement or offset balances automatically.

Only fresh successful Accounts with known complete zero value, no nonzero
Balances, and no Positions, Earn Positions, or Dual Positions are hidden in the
Overview tree. Hidden nodes retain selection, statistical boundary, and history.
Offsetting positive and negative holdings, stale, failed, or unknown Accounts
remain visible. Identity add actions remain available when all Accounts hide.

## Reasons and trade-offs

The Kraken principal omission observed on 2026-10-05 demonstrates why an
API-only total can be incomplete despite successful collection. A small signed
ledger fills that gap without guessing unsupported product interfaces or
introducing transaction accounting. Users remain responsible for quantities and
settlement cleanup; duplicate counting remains possible if they retain already
API-visible amounts. Collection time does not mean quantities were reconfirmed
by the User.

Frozen inputs make retries deterministic and preserve reviewable history. The
pending view and version guard cost extra state but avoid combining uncollected
quantities with old prices or silently overwriting another editor. Adjustment
markers acknowledge that portfolio differences across user edits cannot be
interpreted as uninterrupted financial performance.

Empty-node hiding is presentation only. Keeping uncertain and offsetting
holdings visible prevents a zero subtotal from suggesting absence of holdings.
