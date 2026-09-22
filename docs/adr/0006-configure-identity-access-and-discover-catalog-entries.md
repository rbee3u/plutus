---
status: superseded by ADR-0015 and ADR-0018
---

# Configure identity access and discover catalog entries

Configuration must select which Identities belong to the Portfolio without allowing an Asset or Instrument allowlist to hide value reported for those Identities. Allow at most one enabled Connection for each explicitly enabled Identity, discover its Accounts automatically, and keep discovered subordinate Identities disabled until explicitly enabled. Collect every reported Balance and Position, automatically registering unknown Asset Listings, provisional Assets, and Platform-specific Instruments for later administrative review.

## Consequences

- A Connection is an access path, not an ownership boundary. Credential replacement does not change the Identity, and an older Connection must be disabled before its replacement is enabled.
- Supported means the Platform integration can collect a feature or account mode; Discovered means a catalog entry was observed from the Platform; Tracked means its prices are synchronized; Enabled applies to Connections and Identities included in collection.
- Current nonzero Balances and open Positions automatically determine which Assets and Instruments are Tracked. Users can pin additional catalog entries in their Portfolios without changing Portfolio membership.
- Matching symbols do not merge Asset Listings across Platforms. An unknown listing receives a distinct provisional Asset until a User confirms or revises its mapping within a Portfolio.
- Users can revise canonical mappings, classifications, display information, and Tracked state within their Portfolios while preserving platform-native identities. Mapping and classification revisions remain available to reinterpret historical Observations without rewriting their source provenance.
- An enabled Identity's Accounts are discovered and collected automatically. Unsupported account modes are reported explicitly rather than silently ignored, and discovered subordinate Identities require separate enablement.
