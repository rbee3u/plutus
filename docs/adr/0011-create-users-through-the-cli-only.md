---
status: accepted
---

# Create Users through the CLI only

Public HTTP registration is not part of the product. The operator creates, disables, and recovers Users with `plutus user create`, `plutus user disable`, and `plutus user reset-password`. Login remains a browser Session against those CLI-created credentials.

Keeping a disabled-by-default registration toggle would still require a public API, a Register screen, rate limits, and a cleartext-transport guard for a path nobody is expected to open. Removing the surface keeps the first release local-first and avoids pretending self-serve signup exists. If a later deployment needs open registration, it can add a dedicated contract rather than re-enable a dormant flag.

## Consequences

- CLI user creation still creates the undeletable Portfolio named `default`.
- The JSON API has login, logout, CSRF, password change, and Session management. It has no register or registration-status endpoints.
- The UI has `/login` only. There is no `/register` route or configuration switch for public registration.
