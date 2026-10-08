---
status: accepted
---

# ADR 0019: Manual balances and confirmed empty Accounts

## Decision

Each Identity has a fixed, always-selected Manual Account for signed quantities
missing from the Platform API. Connection health does not remove it from the
Portfolio Boundary; Identity archive does. This extends selected exchange
Accounts without adding external bank assets or transaction accounting.

Edits use a version guard and immutable revisions; changing an Asset code
requires deletion and creation. Each Collection Cycle freezes its Manual inputs
before Platform reads; retries reuse them. Manual uses public own-Platform
prices independently of private collection success. Effective quantity changes
create no Capital Flow and make crossing Value Changes incomplete. See the
[Manual rules](../../CONTEXT.md#manual-balances) for normalization, valuation,
and boundary continuity.

Overview's Identity and Manual headings and the global Balances view share an
editor, available before the first point. The editor reads the current ledger,
including excluded records; pending additions, edits, and deletions appear
separately from collected values. Users remove amounts when they become
API-visible; Plutus does not infer settlement.

Overview hides only freshly confirmed empty Accounts under the domain rules.
Selection, history, and Account counts remain unchanged; the Identity add action
remains available even when all Accounts hide.

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
