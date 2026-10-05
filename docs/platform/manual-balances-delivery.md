# Manual balances delivery report

Delivery date: 2026-10-05. The accepted scope is recorded in
[ADR 0019](../adr/0019-manual-balances-and-empty-accounts.md).

## Recorded evidence and limits

No new responses were promoted or existing evidence replaced. The Kraken
[principal omission check](./kraken/verification.md#dual-investment-contract-review)
at 08:02–08:03 UTC on 2026-10-05 observed USD 33.7841 in Main and BalanceEx,
with no corresponding USD principal in Earn. Combined with the owner's 100 USD
subscription confirmation, this establishes that narrow omission. It does not
validate full totals, settlement, or all Account Structures against Kraken Pro.
The review groups remain temporary and separate from automated tests.

## Independent synthetic coverage

Binance, OKX, Bybit, and Kraken reuse their public market reads through a
credential-free capability. Each Platform has an independent loopback fixture
regression for Manual-only symbols, deduplication, unlisted symbols, stablecoin
omission from market calls, and absence of private preflight. Existing fixture
suites remain independent of recorded evidence. Real public failure and market
availability are not inferred from synthetic success.

Application regression coverage includes versioned CRUD, ownership, CSRF,
normalization, decimal precision, duplicate refusal, zero deletion, audit and
immutable revisions, two concurrent editors, frozen retries and edits during
public reads, signed entry/exit, stale private failure isolation, Manual-only
public pricing, own-Platform restrictions and the 24-hour/future price bounds,
price-only changes, edit-and-undo, history rebuilds, populated upgrades, and
confirmed-empty/offsetting/position-bearing tree behavior.

## Verification status

The ordered gates passed:
`make format && make web && make build && make test && make lint`.
Database-backed Go regressions used throwaway `plutus_gotest_*` databases; all
90 Vitest tests passed. The full Chromium suite passed all 44 tests, including
desktop and mobile Manual flows, conflicts, pending deletions, and entry before
a Valuation Point. The final Linux amd64 build passed. Regenerating SQL and API
bindings followed by formatting left all 24 generated files unchanged. Markdown
formatting, local paths and heading anchors, and `git diff --check` passed.

The embedded SPA was exercised by clicking and submitting on the isolated
`:8081` agent instance with fake Adapters and `plutus_agent`, without loading
recording credentials or accessing real Platforms. All three add entries opened
the shared editor. A disabled Kraken Connection retained stale Main holdings
while Manual USD 100 valued at 100 USD. Saving 101 kept the collected quantity
at 100 and displayed 101 separately; reopening the editor read 101. Deleting the
record displayed a pending zero while preserving the old point.

Before any point, the global entry required an Identity and accepted USD -100
without inventing a value. A new Identity with no selected exchange Accounts had
exactly one selected Manual Account and no credentials. Its first cycle produced
a complete -100 USD point, with user edit time separate from collection time.
After deletion and another cycle, its complete zero-valued Account was hidden
while the Identity add action and Account count remained. Archived Identities
were excluded from the editor's choices. Temporary test Identities were archived
after verification, preserving their history.

All session-owned agent process groups were stopped. The health endpoint became
unreachable and port 8081 had no listener; the human's port 8080 was untouched.

The development runtime supplies no real exchange-app acceptance evidence.

## Remaining real coverage

Full exchange-app reconciliation, nonempty holdings in unexercised structures,
product lifecycle and settlement transitions remain outstanding as documented in
each Platform's verification notes. Manual amounts are owner-maintained; Users
must remove or adjust them when the API starts including those amounts.
