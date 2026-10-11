# Development walkthrough scenarios

The offline dataset has one User: `alice` / `Alice=123456`. Values, rates,
references and price curves are synthetic; native-looking symbols describe
business shapes, not a claim that a product was listed or held on an exchange.
This dataset does not establish exchange-app validation.

Use `db seed` on an empty migrated dev database or an explicit `db reset --seed`
to replace disposable data. Seeding requires the fake Adapter and a master key.
Agents use only `plutus_agent` on :8081. There are no seed profiles.

## Portfolios

| Name         | Timezone      | History                                                   | Purpose                                                             |
| ------------ | ------------- | --------------------------------------------------------- | ------------------------------------------------------------------- |
| `default`    | Asia/Shanghai | 3 months; pagination fixtures join in the final two hours | Healthy holdings, financial events, all margin modes and pagination |
| `edge-cases` | Europe/Berlin | 3 months with an intentional empty day                    | Failures and policy/boundary transitions                            |

Long histories have daily older points and two-hour points in the last three
local days. Event instants have surrounding points. The current local day is
included; the number of local date labels depends on the calendar months at the
anchor. Nonempty Portfolios collect every 30 minutes. A collection does not
advance a special simulation counter; observations depend on the original anchor
and actual time. The earliest partial calendar month or quarter can be
incomplete because its first dates predate the seed horizon; the history read
names those missing dates. Dates inside the generated daily range remain
covered.

## Healthy walkthrough

`default` starts with six independent Platform logins: Binance Classic and
Portfolio Margin, OKX Multi-currency Margin, Bybit Cross Margin, Kraken Separate
wallets and Unified Wallet. All their supported Accounts are selected. Six
additional logins join in the final two hours for large holdings and the other
OKX and Bybit modes; Spot mode has no contract Positions. Their boundary entries
are explicit Value Change events. The former `large` and `empty` Portfolios are
no longer seeded. Create a Portfolio to walk through empty state.

Expand each Identity and Account in Overview, then visit Balances, Futures, Earn
and Dual Investment. Verify known zero USD-equivalent balances, tiny SHIB
quantities, negative borrowed USDT equity, linear and inverse contracts,
opposing hedged holdings, one-way delivery contracts, and margin provenance.
Earn includes product/strategy/position granularity, multi-Asset strategies,
qualified rates and rewards, and distinct maturity and redemption information.
Dual keeps subscriptions separate, including principal in BTC and USDT.

The table uses `D` for the rounded UTC seed anchor, not the browser's current
time. Find exact timestamps in Capital or Activity; History uses the Portfolio
timezone for labels.

| When            | Event                                   | Expected result                                                             |
| --------------- | --------------------------------------- | --------------------------------------------------------------------------- |
| D−60 days       | Binance Spot receives 0.1 BTC           | Spot BTC increases by 0.1; a valued contribution offsets it in Value Change |
| D−30 days       | Binance Funding receives 5,000 USDT     | Funding increases by 5,000 and Capital records the contribution             |
| D−8 to D−4 days | Historical ETH hedge opens, then closes | The Position disappears after success; realized PnL remains in collateral   |
| D−6 to D−5 days | Bybit temporary outage                  | History carries compatible reports as stale; current Bybit is healthy       |
| D−5 days        | Binance Funding withdraws 1,000 USDC    | The Balance decreases by 1,000 and Capital records a withdrawal             |
| D−3 days        | 750 USDT moves Funding → Spot           | Total principal is unchanged; no Capital Flow is invented                   |
| D−3 days        | Named Earn/Dual terms return principal  | Product rows disappear after success; 400/200 USDT lands in Funding         |

Kraken has no Dual Account; its Earn redemption returns 400 USDT to Derivatives
Holding. Ordinary rolling 28-day cohorts keep subscription and entry dates
stable within a cohort. Contract price waves repeat at cohort boundaries;
closing and reopening there does not discard an accumulated gain or loss.

## Edge walkthrough

Use a seven-day History window and `edge-cases` Overview/holdings. Failures
retain compatible source observation times; they never fabricate an empty
successful report. Unknown amounts have omissions rather than zero.

| Identity or location                      | Expected observable behavior                                                                            |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Binance unpriced airdrop                  | Fresh incomplete calculated value; named `DEMO-NOPRICE` omission and unvalued contribution at D−5       |
| Binance delayed COIN-M                    | Stale but known Account value and Futures rows since D−2                                                |
| Binance stale unpriced holdings           | Stale and incomplete are visible together                                                               |
| Bybit never collected                     | Missing Account without compatible success; no invented Positions                                       |
| OKX revoked credentials                   | Invalid Connection, retained stale balances and products                                                |
| Kraken collection paused                  | Disabled Connection with compatible stale Main report                                                   |
| Binance awaiting credentials              | Unconfigured Connection and missing selected Spot Account                                               |
| Binance unavailable core equity           | Normalization failure; no invalid Balance row is stored                                                 |
| Binance retired sub-account               | Archived at D−2, real earlier reports and boundary withdrawals                                          |
| Binance wallet boundary                   | Funding enters D−5; Spot exits D−3; other unselected Accounts remain visible in configuration           |
| Binance reviewed asset policy             | `DEMO-BTC` maps to BTC; `DEMO-DUST` is excluded D−6, restored D−4, excluded D−2; audit gives the reason |
| OKX optional metadata unavailable         | Known value with row-local optional Earn/Dual and Position problems                                     |
| Bybit observed mode change                | Cross → Portfolio Margin at D−3; historical reports retain their own mode                               |
| Binance successful empty wallets          | A genuine empty Complete Set, distinct from missing reports                                             |
| OKX unsupported product report            | Enabled Connection, unsupported Account outcomes and compatible stale history                           |
| History: Berlin local date containing D−4 | No points, null Period Close and explicit missing-day reason                                            |

Compare ranges spanning the unvalued flow, boundary and exclusion changes. The
results remain incomplete and name the reasons. Longer windows exercise quarter,
month, ISO week and Berlin DST boundaries when present in the three-month range.
Controlled-anchor tests cover year and DST changes. The complete daily Portfolio
supplies ordinary comparisons with both gains and losses.

## Pagination and continued collection

In `default`, visit each of the four holdings tables, press Next, and check that
rows continue beyond 50. Filter by Binance's diversified Identity or a symbol,
return to page one, and remove the filter. Repeat for both Catalog lists.
Activity Audit has 60 independent asset mapping reviews, each with actual
resource IDs and before/after mapping state; follow its second page.

The pagination fixtures have only a few recent source sets; repeating their
large holdings over many historical cycles adds reset cost without helping the
walkthrough. This sampling choice does not change the application retention
policy.

Click Collect now and repeat the checks. Product details, large row counts,
configured modes, the unpriced airdrop, persistent Account failures remain
available. Historical recoverable outages remain in History. A newly created
empty Portfolio is not scheduled until an enabled Connection is configured.

After the anchor, only Platform observations evolve. No fake background hook
creates Capital Flows, changes selection, archives Identities or edits platform
asset policy. Owner actions through the normal UI still work and affect
collection.

## Verification contract

Automated tests apply source SQL and initialize points in a throwaway database,
initialize real Connection lifecycle services, initialize immutable valuations,
read actual service results, and collect again through the same scenario
generator. They cover deterministic IDs/output, exact event deltas, source
amounts, calendar/DST timing, compatible stale fallback, normalized product
descriptions, and pagination. Every derived edge-case row is compared with
sequential collection valuation of the same source facts across multiple history
batches, and retries return existing points unchanged. Database tests also cover
interrupted collection cycles, nonmonotonic evaluation times, price fallback
ties, cancellation, partial progress and transactional rollback after a later
report batch fails.

Implementation boundaries and reset policy are recorded in
[ADR 0010](./adr/0010-environments-and-configuration.md). Runtime verification
uses [the agent workflow](./testing.md#agent-runtime-verification); successful
offline fixtures are not a substitute for that check or real exchange-app
comparison.

## Recorded agent check: 2026-10-05

This record describes the previous baseline. Its retired scope-transition and
legacy Earn-summary scenarios are not part of the current dataset. Current
acceptance is recorded in the
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

The current dataset keeps two Portfolios and three months of daily coverage,
with dense sampling reduced to the last three local days. Pagination Identities
join `default` in the final two hours, retaining two or three historical cycles
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
