---
status: accepted
---

# Capitalization

This guide owns capitalization of project-owned English in documentation,
comments, UI, API descriptions, diagnostics, CLI output, scripts, configuration
examples, tests, and seed fixtures. It also defines how prose relates to code
and stored names. Other project guides link here rather than maintaining
separate case rules.

[Glossary](../GLOSSARY.md) owns vocabulary and meaning. This guide owns case,
not new domain definitions, financial behavior, or naming-driven API changes.

## Decide by purpose

Classify the text before editing it. Its file extension and whether it is inside
a string literal do not determine its purpose.

| Purpose                                                            | Convention                                                                 | Example                                                |
| ------------------------------------------------------------------ | -------------------------------------------------------------------------- | ------------------------------------------------------ |
| Ordinary prose and explanatory comments                            | Normal English sentences; common nouns lowercase                           | `Each collection cycle creates a valuation point.`     |
| Titles, headings, UI labels, table headers, and standalone choices | Capitalize the first word and preserve required names                      | `Collection cycles`, `Canonical asset`, `Cross margin` |
| Glossary entry names                                               | Lowercase common-noun terms                                                | **collection cycle**, **account report**               |
| User-facing messages, including API-supplied explanations          | Sentence case; complete sentences end with punctuation                     | `Enter a valid quantity.`                              |
| Internal error chains                                              | Lowercase initial ordinary word; no terminal punctuation                   | `load account report: %w`                              |
| Structured log event messages                                      | Lowercase initial ordinary word; short phrase without terminal punctuation | `collection cycle failed`                              |
| Code identifiers and machine-readable keys                         | Language and contract conventions                                          | `PortfolioID`, `portfolioId`, `portfolio_id`           |
| Official names, literal references, user content, and evidence     | Preserve the required or original spelling                                 | `OKX`, `PnL`, `bn-main`, `If-Match`                    |

Use lowercase common-noun domain terms by default in running prose. That is a
project writing preference, not a requirement of domain-driven design or a claim
that defined-term capitalization is invalid. Required names and sentence starts
retain their normal case.

## Domain terms and defined names

Domain specificity does not by itself make a noun a proper name. Defined-term
capitals are a valid editorial convention, but Plutus uses ordinary common-noun
case to keep explanatory prose consistent across product and technical writing.

For Plutus, prefer ordinary common-noun case in prose and glossary entries. Keep
the glossary's vocabulary and semantic distinctions. Write
`the selected account` or `the platform login` when the intended scope needs
clarification; do not depend on `Account` versus `account` to establish it.
Refer to actual code types with their exact spelling, such as `AccountReport`.
`Account Report` is a capitalized phrase; `AccountReport` is an identifier. They
are not the same notation. Model diagrams can name exact types, with that role
made explicit.

This preference is not backed by a Plutus-specific readability study. Sentence
case for headings and labels alone does not settle whether running prose uses
defined-term capitals; the common-noun rule above makes that separate choice.

## Documentation and comments

Use sentence case for document titles, section headings, captions, diagram text,
table headers, list items, link labels, and standalone table cells. Preserve
literal values and official names inside them. In running text, use `portfolio`,
`identity`, `account`, `account structure`, `margin mode`, `account report`,
`collection cycle`, `valuation point`, `capital flow`, and `boundary event`. The
same treatment applies to all other ordinary glossary terms, including
`manual account`, `earn position`, and `dual investment`.

Glossary entry names are vocabulary entries, not sentence headings: use
**collection cycle**, followed by a definition written as a normal sentence.
Section headings still use sentence case. Bold text does not change case.

Distinguish a concept from its exact identifier. Write `the account report` for
the concept and `AccountReport` for the Go type. Use code formatting for literal
identifiers, paths, commands, flags, field names, and raw values. Preserve their
exact spelling even at the start of a sentence; sentence case does not override
a name or literal. Prefer a short rewrite when a lowercase start looks awkward:
The `plutus` command serves the application. `Plutus` names the product;
`plutus` names its executable.

Code formatting does not exempt illustrative prose or formulas from ordinary
common-noun case; only actual names and literals retain their required spelling.

Go doc comments start with the exact declared name: `// PortfolioID ...` or
`// newPriceBook ...`. Package comments start with `Package logging ...`.
Ordinary comment sentences use normal English case. Preserve the exact syntax of
directives such as `//go:embed`, `//nolint`, SQL `-- name:`, and shell shebangs.
TypeScript documentation follows the same prose rule while retaining exact code
references.

In a heading with a colon, capitalize the first word of the subtitle:
`History: Valuation points`. After a colon in ordinary prose, continue in
lowercase unless a proper noun, literal, or quotation requires otherwise. An
independently rendered label/value pair retains each value's own convention, as
in `Status: Active`. A hyphen does not introduce title case: use
`Read-only access` and `Multi-currency margin`.

## UI and human-readable output

Use sentence case across navigation, browser titles, dialogs, buttons, fields,
options, badges, tooltips, empty states, chart labels, accessible names, and
authored API display metadata. Backend-provided display copy follows the same
rule as frontend-authored copy. Define labels at their source.

A standalone choice is `Buy low`, `Sell high`, `Active`, or `Unknown`. An
ordinary word embedded in a phrase is lowercase: `Search asset`,
`Filter platform`, `The account is active.` A label beginning with an
abbreviation keeps it: `API key`, `USD value`. A full sentence starts with a
capital even in a tooltip; a short supporting fragment may continue its
surrounding phrase.

Count phrases keep ordinary words lowercase after the number: `1 issue`,
`2 stale`, and `3 incomplete`. Standalone status labels remain `Stale` and
`Incomplete`. Review the composed text, not just each interpolation separately.

When referring to a visible control or named account, reproduce its actual
label: `Select Collect now.` and `Balances of Spot`. When describing the
concept, use `run a collection cycle` or `the spot account` as appropriate. This
exception does not promote every occurrence of a page name's words to a proper
noun.

Labels and short headings have no final period. Explanatory sentences do. Do not
use uppercase or lowercase CSS transforms to restyle authored text. Do not apply
a general title-case or sentence-case function to dynamic content. Finite,
application-owned enum labels can use explicit mappings or a narrowly scoped
formatter when their vocabulary and required spelling are known.

CLI command summaries, flag descriptions, interactive prompts, script guidance,
and ordinary completion messages use sentence case. Command names, flags,
metavariables, generated Cobra sections, tabular machine output, and structured
diagnostics retain their separate conventions. For example, `Created user ...`
is prose, whereas `--username` is a literal. A description beginning with the
command name keeps `plutus` lowercase; alternatively, write
`The plutus command ...`. Plain-text help preserves literal spelling without
adding Markdown backticks. Do not recase values interpolated into the message or
replace framework-generated help templates solely to change their
capitalization.

This policy does not alter numeric precision, currency notation, chart axis
alignment, or units. Those rules remain in
[presentation](./presentation.md#numeric-presentation). Established tokens such
as time-window presets `1D` and `1M` keep their defined case; they are not words
to restyle.

## Errors and diagnostics

### Internal errors

Go errors intended for wrapping and technical diagnosis use a lowercase initial
ordinary word and no terminal punctuation. Use `read account report: %w`, not
`Read Account Report: %w.` Proper nouns and abbreviations keep their spelling:
`TLS certificate is expired` and `Binance request failed` are valid.

Keep error kinds, codes, and identities independent of message spelling. Do not
derive classification or control flow from rendered English.

### User-facing explanations

API problem titles use sentence case without a final period: `Not found`.
Authored problem details, validation feedback, collection issues, and friendly
adapter explanations use complete sentence-case sentences. They do not adopt the
lowercase error-chain style merely because Go carries them through `error`. For
example, a user-safe application rejection is `Enter a valid quantity.`

An error message that serves both a technical caller and a UI must be classified
by its intended destination. Application errors already designated as user-safe
follow the user-facing rule. Internal causes retain technical formatting. Do not
solve this distinction by capitalizing every `err.Error()` at the HTTP or
browser boundary; that can corrupt literals and obscure source provenance.

Third-party errors, source diagnostics, parser output, and historical persisted
messages preserve their source spelling. If a friendly explanation is needed,
author it separately without rewriting the underlying diagnostic. Existing
payload, credential, and logging restrictions remain in force; a capitalization
review does not authorize exposing additional data.

### Logs

Keep project-authored structured event messages as concise diagnostic phrases:
`collection cycle failed`, `HTTP request`, `SQL execution`. Preserve acronyms,
brands, identifiers, error values, and source messages. Log severity and
built-in handler fields retain the logging library's spelling, such as
`level=INFO`. Lowercase diagnostic messages are a project preference, not a
requirement of Go or OpenTelemetry.

Project-owned attribute keys use lowercase words joined with underscores, as in
`identity_id` and `error_class`. Values follow their own contracts. SQL text,
HTTP methods, stack traces, bridged framework output, and error attributes are
not prose to normalize. Shell fallback diagnostics implementing the same logging
role follow this convention; ordinary user instructions follow the UI rule.

## Names, abbreviations, and source text

| Category                                           | Required treatment                                                                                                                                                                              |
| -------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Brands and tools                                   | Preserve `Plutus`, `Binance`, `OKX`, `Bybit`, `Kraken`, `Go`, `TypeScript`, `React`, `MySQL`, `sqlc`, `macOS`, and other official spellings.                                                    |
| Technical and financial abbreviations in prose     | Use `API`, `HTTP`, `HTTPS`, `URL`, `ID`, `UUID`, `JSON`, `SQL`, `TLS`, `CSRF`, `USD`, `APY`, `APR`, `PnL`, `IMR`, `MMR`, and `ISO`; expanding an abbreviation does not capitalize common nouns. |
| Compact account aliases                            | Preserve the aliases owned by [presentation](./presentation.md#account-aliases), including `MCM`, `PM`, and `ISO`.                                                                              |
| Ordinary account and product categories            | Use `Spot`, `Futures`, `Earn`, `Manual`, `Portfolio margin`, and `Dual investment` as standalone display labels; lowercase ordinary words in prose.                                             |
| Official product names and reported product titles | Preserve supplied names such as `Unified Trading Account 2.0` and `Simple Earn Flexible`. Do not assume every capitalized exchange category is a brand.                                         |
| Asset and contract symbols                         | Use the existing normalization and source-preservation rules in [architecture](./architecture.md#persisted-asset-references); preserve visible codes such as `BTC` and `BNBUSDT`.               |
| User-provided text                                 | Preserve display spelling for names, notes, and other content. Comparison keys retain their existing case-folding policy.                                                                       |
| External quotations and recorded evidence          | Preserve original case, including source API keys, enum values, messages, URLs, and product metadata.                                                                                           |

When an official name and an application-owned category resemble each other,
classify the specific field: a reported product title is source content; the
account catalog's display label is authored product copy. Verify uncertain
official spellings against the relevant primary source before adding an
exception. For example, Binance's named `Multi-Assets Mode`,
`Single-Asset Mode`, and `Portfolio Margin Pro` keep their official case. A
wrapped name retains its spelling across lines. Technical references such as the
`User-Agent` HTTP header also keep their conventional spelling in prose, even
where the protocol accepts case-insensitive matching. The glossary continues to
own term choice; this table owns only case.

## Code and machine contracts

Sentence case applies to English, not to identifiers. Keep these conventions:

| Surface                                     | Convention and examples                                                                                                                                                                                                                                                                              |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Handwritten Go                              | Exported `PortfolioID`, `ValueUSD`, `ServeHTTP`; unexported `portfolioID`, `valueUSD`; package names lowercase. Use Go initialisms consistently.                                                                                                                                                     |
| Handwritten TypeScript                      | Types and React components `PortfolioSettings`; functions, hooks, variables, and properties `formatUsd`, `usePortfolio`, `portfolioId`. Treat embedded abbreviations as words: `Id`, `Usd`, `Api`, `Url`. Preserve required platform APIs such as `URLSearchParams`.                                 |
| TypeScript constants                        | Module-level fixed constants may use `NO_POINT` or `PERIOD_KINDS`; ordinary values and local bindings use lower camel case. `const` alone does not require uppercase. React context/component bindings keep their established capitalized names.                                                     |
| JSON/OpenAPI properties and operations      | Preserve lower camel case, such as `portfolioId`, `totalUsd`, and `listPortfolios`. Schema names retain their existing generator convention. Descriptions and summaries are prose.                                                                                                                   |
| Stored codes, SQL names, and log attributes | Lowercase words joined with underscores, such as `capital_flow`, `account_reports`, and `identity_id`, unless an existing contract says otherwise. SQL keywords remain uppercase.                                                                                                                    |
| Configuration and commands                  | Preserve existing TOML keys and lowercase command/flag spellings; environment names remain `PLUTUS_MASTER_KEY`. Make targets and tool directives retain their exact case.                                                                                                                            |
| Files and CSS                               | Preserve local conventions: `capitalization.md`, `account_report.go`, `PortfolioSettingsPage.tsx`, `usePlatforms.ts`, `styles.module.css`, camel-case CSS module classes, and lowercase hyphenated CSS custom properties. Conventional files such as `README.md` and `AGENTS.md` retain their names. |
| Tests, seeds, and examples                  | Authored descriptions and comments follow prose rules; values modeling user content, native payloads, or machine contracts retain representative spelling. Exact-copy assertions follow the approved authored copy.                                                                                  |

Generated bindings retain generator output, including Go names such as
`PortfolioId`, despite the handwritten Go convention. Update owned OpenAPI or
SQL descriptions at their source and regenerate. Do not manually restyle
generated files, vendored dependencies, lockfiles, cached skills, or external
plugin content. Generator-owned diagnostics are an implementation exception;
application-owned friendly messages still follow this guide.

Case cleanup must not rename wire fields, stored codes, tables, configuration
keys, routes, environment variables, external source fields, or exported APIs as
a side effect. Any necessary contract change needs its own compatibility review.
Historical records, audit payloads, recorded responses, and existing migrations
are not rewritten to match a newer editorial preference.

## Repository metadata and test names

Issue and PR prose, release notes, and authored review comments follow the
documentation convention. Preserve the existing Conventional Commit format:
`fix(web): clarify issue labels`. Type and scope remain lowercase; the subject
starts with a lowercase ordinary word and preserves names and abbreviations. A
PR title may use that same format when it is intended as the commit title; other
PR and issue titles use sentence case. Do not rewrite Git history.

Go test function identifiers follow Go conventions. Human-readable standalone
test titles use sentence case. Nested Go subtest paths and table-case keys may
use lowercase diagnostic fragments, such as `missing price`. BDD descriptions
that continue `it` or `describe` may retain lowercase fragments, as in
`it("renders active state", ...)`. Symbol-named suites preserve the symbol:
`describe("TableFilterHeader", ...)`. Exact message assertions and fixtures
retain the case of what they represent, including deliberate mixed-case inputs
used to test normalization. Test failure diagnostics may use lowercase phrases;
names and captured values remain exact.

## Adoption and review

Apply this policy to current project-owned documents and ADR prose, comments,
OpenAPI descriptions, UI metadata, CLI text, logs, friendly errors, scripts,
configuration comments, and authored fixtures. Keep source facts and machine
contracts intact. Regenerate affected outputs and update meaningful copy
assertions. Track rollout evidence outside this normative guide.

Before approving a change, check the text's purpose, common-noun usage, literal
exceptions, composed labels, accessibility names, and any generated source.
Regex searches can identify candidates but cannot decide whether a word is a
proper noun, a code token, or a quote. Prettier, ESLint, and current Go lint
rules do not enforce this whole policy. Prefer review and focused checks over
blanket case conversion; add automated enforcement only for unambiguous
recurring rules. Follow the existing [verification requirements](./testing.md),
including full Go gates for Go comments and agent runtime checks for UI or API
changes.

## Rationale and sources

Reviewed on 2026-10-15. These references inform this guide; they do not impose
one universal convention across prose, programming languages, and diagnostics.

- [Microsoft capitalization](https://learn.microsoft.com/en-us/style-guide/capitalization)
  and
  [Carbon content](https://www.carbondesignsystem.com/building-blocks/foundations/content)
  support sentence case for interface labels and headings.
- [Google capitalization](https://developers.google.com/style/capitalization)
  supports lowercase common-noun glossary entries and discourages conveying
  semantic distinctions through capitalization alone.
- [Google product names](https://developers.google.com/style/product-names)
  preserves official spelling even at sentence starts; its
  [code-in-text guidance](https://developers.google.com/style/code-in-text)
  distinguishes literals from surrounding prose.
- [Eric Evans's DDD Reference](https://www.domainlanguage.com/wp-content/uploads/2016/05/DDD_Reference_2015-03.pdf)
  calls for shared model-based language without prescribing a universal case
  rule in its ubiquitous-language section. Defined-term capitals have direct
  precedents in
  [Mathias Verraes's guidance](https://verraes.net/2011/05/ubiquitous-language/)
  and [Nick Chamberlain's modeling text](https://buildplease.com/pages/fpc-2/).
  Plutus chooses the common-noun alternative; capitalization is not a DDD test.
- [Go review guidance](https://go.dev/wiki/CodeReviewComments#error-strings)
  distinguishes composable error strings from logging and specifies Go
  [initialisms](https://go.dev/wiki/CodeReviewComments#initialisms).
  [Go doc comments](https://go.dev/doc/comment) explain declaration-led
  comments.
- [Google's TypeScript naming guidance](https://google.github.io/styleguide/tsguide.html#naming)
  treats abbreviations as words in identifiers. This project adopts that aspect,
  not the whole Google guide or its file-naming conventions.
- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html#section-3.1) separates
  human-readable problem explanations from machine-readable problem identity.
  Its structure does not prescribe this project's capitalization.
- [OpenTelemetry's log data model](https://opentelemetry.io/docs/specs/otel/logs/data-model/)
  separates message bodies, severity, and attributes. The lowercase event-phrase
  convention above is a deliberate continuation of local practice.
