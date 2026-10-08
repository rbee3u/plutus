---
status: accepted
---

# Feature ownership and shared browser mechanisms

Organize the browser into app shell, product features, shared mechanisms/UI, and
API access. Features own their internals and use explicit cross-feature
entrypoints; shared code does not depend on features or app. API consumers use
its public entrypoint. Keep styles beside their owner and common styles shared;
compound selectors and cascade-sensitive rules stay within one CSS module.

QueryClient owns the authenticated User and Portfolio list. Shared mechanics
handle authentication, CSRF, cache keys, invalidation, and navigation; features
retain SDK results and their conflict, validation, idempotency, and editor
state. Holdings owns view memory; Settings owns Portfolio creation; Shell owns
selection and navigation. Password business rejections do not end Sessions.
Session lists load on entry and revocation without background refresh.

Split components by responsibility, not line count. Holdings shares filters,
recovery, summaries, pagination, and report reads while product pages retain
columns, calls, totals, and empty states. Manage uses finite credential metadata
from the Platform catalog without a general form engine; absent catalog data
requires recovery. Browser tests share session routing and typed fixtures while
owning their financial state and assertions; unknown mock requests fail.

Feature ownership keeps related behavior and styles together. Explicit exports
and coarse lint checks limit accidental coupling without imposing a per-file
matrix. Shared mechanics avoid repeated infrastructure while leaving product
policy local; costs include uneven component sizes, deliberate CSS boundaries,
and care when shared classes gain new audiences. A universal controller, form
engine, or mock framework would obscure these differences.

[Architecture](../architecture.md#frontend-structure) owns mechanisms and
[testing](../testing.md) owns enforcement. [Presentation](../presentation.md)
owns user-facing rules, including refresh and URL state.
