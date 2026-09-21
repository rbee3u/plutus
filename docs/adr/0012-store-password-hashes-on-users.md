---
status: accepted
---

# Store password hashes on users

The Argon2id PHC string is a required field of a User. A 1:1 `user_credentials` table added no extra identity, policy, or audit behavior, and `password_changed_at` was never read. `users.password_hash` is `NOT NULL`. Create, change, CLI reset, and login rehash all write the hash and bump `users.updated_at`. Session revoke still happens on change and reset, not from a password-changed timestamp.

Keeping a satellite table would only preserve a join that every login paid for. If a later release needs password history, lockouts, or a change timestamp, those belong in a purpose-built table rather than restoring this 1:1 row.

## Consequences

- Goose `20260920120015` copies hashes onto `users`, then drops `user_credentials`. The original auth migration is not rewritten.
- Seeds insert `password_hash` on `users`. There is no `user_credentials` seed row.
- Down recreates a 1:1 `user_credentials` row and copies `users.updated_at` into the old `password_changed_at` column.
