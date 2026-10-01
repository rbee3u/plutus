---
status: accepted
---

# ADR 0007: CLI-provisioned Users and write-only credentials

## Decision

Provision, disable, and recover Users through the CLI; offer browser login,
password change, and Session management without public registration. Store
Argon2id password hashes on Users and hashed random Session tokens in the
database. Password change and reset revoke Sessions.

Encrypt Connection credentials under a separately managed master key and never
return stored secrets to the browser. Binance accepts HMAC secrets or
unencrypted PKCS#8 Ed25519 private keys; signing type is derived from the
secret, not an Account setting. Credentials remain bound to the Identity's
external reference.

## Reason and trade-offs

CLI provisioning avoids an unused public signup surface and an application role
model. Keeping a User's single password hash with the User avoids a separate
identity with no independent lifecycle. Write-only encrypted credentials reduce
exposure but require safe master-key backup and re-entry when secrets change.
Synthetic signing checks do not establish live Ed25519 access.

Browser and transport protections are in
[architecture](../architecture.md#authentication-and-secrets); operational
provisioning is in [development](../development.md).
