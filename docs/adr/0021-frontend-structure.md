---
status: accepted
---

# ADR 0021: Frontend feature layout and shared mechanisms

## Decision

Separate `web/src` into `app` (shell, routes, theme), `features` (product
areas), `shared` (common mechanisms and UI), and `api` (client, requests, cache
keys). Create feature subdirectories only when needed. Manage owns the Data
navigation group and retains `/data/...` URLs.

Features use their own internals, shared code, and explicit cross-feature
entrypoints. They do not import the shell; shared code does not import features
or app. API consumers use `api/index.ts`. A local ESLint rule checks resolved
imports, re-exports, and literal dynamic imports; within-feature dependencies
remain a review concern. App may assemble feature pages directly.

Keep feature styles beside their owner and common styles in `shared/styles`.
Compound selectors must stay in one CSS module because hashing is per file.
Co-locate rules whose relative cascade order matters instead of relying on
cross-module import order.

Split components at distinct responsibilities, not line-count thresholds.
Holdings shares filters, recovery, summaries, pagination, and Account Report
reads; pages retain product-specific calls, columns, totals, and empty states.
Settings owns the Portfolio creation form; Shell owns selection and navigation.
Security separates password fields and Session reads.

Manage renders credential inputs from the Platform catalog's finite form
description while retaining validation, ETag, and replacement state. Missing
catalog data offers recovery rather than guessing a form. Browser tests share
session routing and typed fixtures, while each suite owns its financial state,
failures, and assertions. Unknown mock requests fail.

## Reason and trade-offs

Feature ownership makes pages and their styles easier to change together.
Explicit exports keep shared mechanisms distinct from reusable business
presentation. Common classes must move when their audience changes, and CSS
module boundaries require care with compound selectors and cascade order.

Cohesive components can have uneven sizes. A generic list controller, form
engine, or mock-server framework would obscure the product differences these
small shared mechanisms preserve. Coarse lint checks replace the earlier
review-only preference without prescribing every internal dependency.

[ADR 0022](./0022-application-boundaries.md) owns shared data/request lifetimes;
[architecture](../architecture.md#frontend-structure) describes implementation.
