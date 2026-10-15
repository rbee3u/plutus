---
status: accepted
---

# CLI-provisioned users and write-only credentials

Provision, disable, and recover users through CLI operations; browser features
cover login, password change, and session management without public signup or an
application role model. Store Argon2id password hashes with users and hashed
random session tokens in the database. Password change and reset revoke
sessions.

Connection credentials are write-only and encrypted under a separately managed
master key. Their first successful validation binds the identity to an external
login. Signing type is derived from credential material rather than account
settings. Binance supports HMAC secrets and unencrypted PKCS#8 Ed25519 keys.

CLI provisioning avoids an unnecessary public signup surface. Keeping the
password hash with its user avoids a second identity lifecycle. Encrypted,
write-only credentials reduce exposure but require master-key backup and secret
re-entry. Synthetic signing tests do not establish live signing access.

[Architecture](../architecture.md#authentication-and-secrets) owns browser and
transport protections; [development](../development.md) owns provisioning.
