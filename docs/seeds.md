# Development walkthrough scenarios

The offline dataset has one User: `alice` / `Alice=123456`. Values, rates,
references and price curves are synthetic; native-looking symbols describe
business shapes, not a claim that a product was listed or held on an exchange.
This dataset does not establish exchange-app validation.

Use `db seed` on an empty migrated dev database or an explicit `db reset --seed`
to replace disposable data. Seeding requires the fake Adapter and a master key.
Agents use only `plutus_agent` on :8081. There are no seed profiles.

## Portfolios

| Name         | Timezone      | History                                | Purpose                                                        |
| ------------ | ------------- | -------------------------------------- | -------------------------------------------------------------- |
| `default`    | Asia/Shanghai | 3 months                               | 12 healthy active Identities and one archived historical login |
| `edge-cases` | Europe/Berlin | 3 months with an intentional empty day | 3 Identities covering four representative valuation states     |

Long histories have daily older points and two-hour points in the last three
local days, with brackets around event instants. The current local day is
included. Collection runs every 30 minutes. Observations depend on the original
anchor and actual time; collection does not advance a simulation counter.
Partial calendar periods can be incomplete when their opening dates predate the
seed horizon; History names those missing dates.

## Healthy walkthrough

Identity names and account combinations borrow the shapes of a locally inspected
production dump from October 11–13, 2026. Generic labels such as `bn-main`,
`ok-main`, `by-main`, `kr-main`, `okxSum`, `okxCoinM`, `okxUsdM`, and `kkSum`
are retained; personal sub-login labels become `bn-sub-01` through `bn-sub-04`.
No production IDs, credentials, sessions, exact balances, or financial records
are imported. The dump's roughly 57-hour history cannot establish the realism of
three-month synthetic curves or long-term failure frequencies.

`default` has four Binance Portfolio Margin logins, one Binance Classic login,
four OKX logins (one Portfolio Margin and three Multi-currency Margin), one
Bybit Portfolio Margin login, and Kraken Separate and Unified logins. Unused
Classic margin wallets are unselected. Many selected wallets are freshly empty.
Balances, hedge sizes, and Earn products differ by login; most value is
concentrated in Earn and trading wallets. Negative stablecoin equity and tiny
balances are normal observations. The aggregate synthetic scale is a few hundred
thousand USD, with one larger main login and several smaller logins.

Current Futures Positions are short cross perpetuals, with both linear and
inverse settlement. Most Earn descriptions are flexible product aggregates;
Kraken retains strategy granularity and bonded/timed descriptions. Dual has just
two current BTC/USDT subscriptions under `okxSum`, with 120 USDT and 0.002 BTC
principal. Other selected Dual wallets are empty. Optional measures stay unknown
where the simulated Platform does not supply them.

Expand Identities and Accounts in Overview, then inspect Balances, Futures,
Earn, and Dual Investment. Create a new Portfolio to inspect empty state.

The table uses `D` for the rounded UTC seed anchor. Find exact timestamps in
Capital or Activity; History labels use the Portfolio timezone.

| When            | Event                                                         | Expected result                                                           |
| --------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------- |
| D−60 days       | `bn-sub-04` Spot receives 0.1 BTC                             | A valued contribution offsets the increase in Value Change                |
| D−30 days       | `bn-sub-04` Funding receives 5,000 USDT                       | Capital records the contribution                                          |
| D−8 to D−4 days | A historical SOL hedge opens and closes in `bn-sub-04` USDⓈ-M | The Position disappears after success; realized PnL remains in collateral |
| D−6 to D−5 days | Temporary Bybit outage                                        | Historical reports are stale; current Bybit is healthy                    |
| D−5 days        | `bn-sub-04` Funding withdraws 1,000 USDC                      | Capital records the withdrawal                                            |
| D−3 days        | 750 USDT moves Funding → Spot                                 | Total principal is unchanged; no Capital Flow is invented                 |
| D−3 days        | `bn-sub-04` Earn/Dual terms return principal                  | 400/200 USDT returns to Funding after the product rows disappear          |
| D−3 days        | Bybit changes Cross → Portfolio Margin                        | Historical reports retain their observed regime                           |
| D−2 days        | `bn-retired` is archived                                      | Earlier holdings and boundary withdrawals remain in History               |

Only `bn-sub-04` owns the scripted investment and transfer events. Ordinary
rolling cohorts keep entry and subscription dates stable; bounded price waves
repeat at close/reopen boundaries. Long-term curves and all scripted financial
events are synthetic.

## Edge walkthrough

Use a seven-day History window and `edge-cases` Overview/holdings. Failures
retain compatible source observation times rather than fabricating successful
empty reports. Unknown amounts have named omissions.

| Identity / Account               | Expected observable behavior                                                                |
| -------------------------------- | ------------------------------------------------------------------------------------------- |
| Binance unpriced airdrop / Spot  | Fresh incomplete calculated value; `DEMO-NOPRICE` omission and unvalued contribution at D−5 |
| Binance delayed reports / COIN-M | Stale but known Account value and Futures rows since D−2                                    |
| Binance delayed reports / Spot   | Stale and incomplete together, with unpriced historical holdings                            |
| Bybit never collected / Unified  | Unknown value without compatible successful history; no invented Positions                  |

The unpriced Identity also supplies historical boundary changes: Funding enters
at D−5 and exits at D−3 while Spot stays selected. `DEMO-BTC` maps to BTC;
`DEMO-DUST` is excluded at D−6, restored at D−4, and excluded at D−2, with audit
reasons. Comparisons crossing the unvalued flow or relevant treatment changes
remain incomplete and name those reasons. The Berlin local date containing D−4
has no points and an explicit missing-day reason.

Invalid/disabled/unconfigured Connections, additional failure classes, optional
metadata failures, all supported modes, and large lists belong to dedicated
fake, application, and pagination tests. They do not inflate the daily seed mix.

## Continued collection and verification

Click Collect now and repeat the holdings checks. `default` stays healthy and
retains its differentiated wallets and sparse products; the four representative
edge states retain their observation times and omissions. Historical recoverable
outages stay in History. After the anchor, only Platform observations evolve;
the fake creates no new Capital Flows, selection changes, archives, or policy
edits. Owner actions through the UI still affect collection.

Automated tests apply source SQL in a throwaway database, initialize real
Connection services and immutable valuations, read the results, and collect
again through the same generator. They cover the normal majority, empty wallets,
product concentration, deterministic output, coherent events, stale fallback,
and calendar/DST timing. Edge history is compared with sequential valuation
across history batches; retries leave committed points unchanged. Separate
fixtures verify every supported mode and second pages of all holdings, Catalog,
and Activity lists. Daily seeds need not have a second page in every list.

Reset policy is recorded in
[ADR 0010](./adr/0010-environments-and-configuration.md). Runtime acceptance
follows [agent verification](./testing.md#agent-runtime-verification). Synthetic
walkthrough checks do not establish exchange-app reconciliation.

## Production-shaped seed and agent check: 2026-10-14

The ordered format, web, build, test, and lint gates passed, including temporary
MySQL coverage and 228 frontend tests. Dedicated fixtures retained the full
capability matrix and second-page checks for all holdings, Catalog, and Activity
lists. No production database connection or dump import was used.

A compiled-binary reset of `plutus_agent` generated source facts in 280 ms,
initialized source facts and valuations in 2.085 seconds, and initialized
Connections in 76 ms. The seed load reported 2.442 seconds, excluding migration
and compilation. This single host measurement is not a performance guarantee.

On the embedded SPA at :8081, Alice switched between both Portfolios. `default`
had 12 active Identities and 67 selected Accounts, with approximately
$266,112
known value, no stale Accounts, and complete valuation. Its current holdings had
72 Balances, 28 Futures Positions, 20 Earn descriptions, and two Dual
subscriptions. The main Binance login concentrated about $64k
in Portfolio Margin and $50k in Earn. Fresh empty wallets stayed hidden from
Overview.

Interaction covered all four holdings views and both manual collections. Normal
collection preserved completeness and product counts. `edge-cases` retained
seven selected Accounts, two stale Accounts, and three incomplete Accounts;
unknown Unified value displayed as an em dash and stale observation times stayed
at their compatible pre-outage reports. History's three-month range remained
available, including the intentional Berlin missing day. Capital showed three
normal flows, the unvalued airdrop, the Funding entry/exit pair, and the
archived login's three boundary withdrawals.

The verification server was stopped, health became unreachable, and :8081 had no
listener. The production dump stayed outside the repository. These observations
validate synthetic walkthrough behavior, not exchange-app reconciliation.

## Recorded agent check: 2026-10-05

This record describes the previous baseline. Its retired scope-transition and
legacy Earn-summary scenarios are not part of the current dataset. Acceptance
for that baseline is recorded in the
[schema delivery report](./platform/schema-provenance-delivery.md).

The ordered format, web, build, test and lint gates passed. On the embedded SPA
at :8081, Alice signed in and switched among all four Portfolios. The check
covered all holdings views, asset filtering, both Catalog lists, audit
pagination, Connection states, archived Identities, Capital Flows, boundary
events, long history and the intentional Berlin empty day. Value Changes named
the unvalued flow, scope transition and asset-treatment changes. The legacy Earn
summary and optional metadata problems remained visible.

Manual collection succeeded for `default`, `edge-cases` and `large`. The default
remained complete; edge failures retained compatible observation times and
explicit omissions. Large holdings retained 116 Balances, 79 Futures Positions,
84 Earn Positions and 89 Dual Positions, with a working second page in each
view. Both Catalog lists and audit history also had second pages. The empty
Portfolio had no Identities or Valuation Points. These observations validate the
synthetic walkthrough, not real exchange holdings or market history. Both
session-started process groups and their children were stopped; agent health
became unreachable and :8081 had no listener.

## Two-Portfolio measurement and agent check: 2026-10-11

That baseline kept two Portfolios and three months of daily coverage, with dense
sampling reduced to the last three local days. Pagination Identities join
`default` in the final two hours, retaining two or three historical cycles
instead of repeating large holdings across a separate seven-day Portfolio. Event
brackets, the Berlin missing day, all Account modes and all pagination views
remain covered. This changes only synthetic sampling, not retention.

Three consecutive compiled-binary `db reset --seed` runs before this reduction
took 7.315, 6.562 and 6.204 seconds. Afterward they took 3.898, 2.938 and 3.019
seconds: the median fell from 6.562 to 3.019 seconds, approximately 54%. These
local runs used `config/agent.toml`, `config/agent.env` and normal SQL logging,
with the server stopped. They include reset, migration, generation, source
insertion, valuation and Connection initialization; compilation is excluded. The
first optimized reset also removed the larger preceding dataset. Timings are
evidence for this host, not a guarantee for other machines.

The ordered format, web, build, test and lint gates passed, including 193 web
tests. API verification on :8081 found 164 `default` points and 157 `edge-cases`
points at this anchor. The latest default remained complete; edge-cases remained
incomplete with explicit omissions. Balances, Futures, Earn and Dual Investment
had 179, 103, 108 and 109 rows, respectively; asset and contract Catalogs had 82
and 105 rows. Every list and the independent audit reviews had a second page.
Counts vary with the anchor. The embedded UI verified login, both Portfolio
choices and their complete/incomplete Overview states. Omitting `cookie_suffix`
from a temporary agent configuration also verified the default
`plutus_session_prod` and `plutus_csrf_prod` cookies without using production.

## Performance measurement: 2026-10-11

This earlier measurement predates the two-Portfolio reduction above.

With normal SQL logging on the agent database, the previous initializer took
9.08 seconds for a complete reset and issued 13,019 SQL statements. Historical
price selection alone issued 4,939 queries. The revised seed-only initializer
loads finite price and exclusion histories once per Portfolio and compares
Manual quantities per 64-point batch. Runtime collection retains its bounded
price reads and ordinary first-commit path.

Three consecutive resets of the compiled working-tree binary took 7.191, 6.038,
and 6.328 seconds: a 6.328-second median, about 30% below the measured baseline.
Source insertion and Point initialization took 5.019, 4.715, and 5.000 seconds.
The statement count fell to approximately 5,514; four bulk price loads replaced
the individual price queries, and Manual comparison/timeline reads fell from
approximately 1,297 to 24. All runs retained the same 650 Valuation Points.
Compilation is excluded; generation, reset, source insertion, valuation, and
Connection initialization are included. These are local measurements, not
performance guarantees for other hosts or arbitrary seed-history sizes.

Price selection parity tests cover Platform isolation, future retrievals,
source-time ordering, expired prices, and nonmonotonic lookups. A 131-cycle
regression covers Manual edits, clears, and stale empty sources across history
batch boundaries. The complete seeded edge-case history still matches ordinary
sequential valuation. No scenarios or history windows were reduced.

## Historical performance measurement: 2026-10-05

These measurements predate immutable valuation and retention. They are retained
as historical evidence, not a performance claim for the current implementation.

After batch rebuild optimization, the original 400-day/45-day-dense dataset
still took 16.94 seconds for a complete reset with normal SQL logging. The
accepted fallback uses three months, a 14-day dense window and the D−60 BTC
deposit. Existing data requires an explicit reset.

Three consecutive runs of the final compiled `dist/plutus-darwin-arm64`
`db reset --seed`, using `config/agent.toml` and `config/agent.env`, took 6.405,
5.519 and 5.630 seconds. SQL logging retained its normal enabled policy. The
median was 5.630 seconds and all runs met the 10-second target; SPA and Go
compilation were excluded. Generation took 0.904–0.916 seconds, source insertion
2.135–2.194 seconds, Connection initialization 0.116–0.124 seconds, and total
valuation rebuild 2.111–2.211 seconds. The source dataset had 286 default, 268
edge-case, 105 large and zero empty Valuation Points at this anchor; counts vary
with local time and event sampling.

The ordered gates and [browser checks above](#recorded-agent-check-2026-10-05)
passed again with three-month history and the D−60 Capital Flow. Collection
preserved the same large-Portfolio row counts, pagination, stale observations,
missing prices, and legacy Earn summaries. Asset filtering reset pagination
correctly. The process group and children were stopped, :8081 was released, and
temporary tooling was removed.
