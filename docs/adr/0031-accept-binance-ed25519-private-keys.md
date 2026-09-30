---
status: accepted
---

# Accept Binance Ed25519 private keys alongside HMAC secrets

Binance supports Ed25519 API keys for signed Spot requests. Plutus has one write-only credential pair per Binance Connection and signs every Binance account call in one client. The owner wants to use an Ed25519 key without changing the Account Structure or selecting a signing mode manually. No Ed25519 read-only test key is currently available.

- A Binance `apiSecret` beginning with a PEM header must be an unencrypted PKCS#8 Ed25519 private key. Reject malformed, encrypted, and other-key PEM at credential storage. A value without a PEM header remains an HMAC secret. Existing encrypted credential documents therefore retain their meaning and need no migration.
- Sign the encoded request parameters with Ed25519, send the Base64 signature as a percent-encoded query parameter, and retain the current HMAC-SHA256 hex signature for other Binance credentials. The shared client applies this to both supported Account Structures and every existing signed endpoint.
- Keep credentials write-only. The Connection response does not expose a signing type; the type is derived from the decrypted secret when signing. The UI accepts multiline PEM and clears it after storage.
- Use synthetic Ed25519 keys with the explicitly selected documentation-derived response samples to test signing offline ([ADR 0027](./0027-allow-documented-adapter-scenarios-without-live-coverage.md)). This does not establish live Ed25519 access to SAPI, PAPI, FAPI, or DAPI. Report that gap until a read-only key exercises those paths.

## Consequences

- No database column, encrypted document version, API field, or Account Type is added.
- The existing `PLUTUS_GOTEST_BINANCE_SECRET` accepts a PEM with newline escapes in the quoted `config/gotest.env` value, or a multiline process environment value, for explicit single-interface recording (ADR 0036).
