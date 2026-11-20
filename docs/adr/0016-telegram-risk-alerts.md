---
status: accepted
---

# Telegram high-leverage alerts

Overview already marks a supported unified account whose known real leverage is
strictly above 3. The owner asked for the same risk to arrive without keeping a
browser open, so the server optionally sends it to a Telegram group.

## Decision

After a successful valuation point commits, the collection runner invokes an
optional notifier with the portfolio and cycle. The notifier reads the point's
account valuations and each supported account's stored positions and applies the
same predicate as Overview: a known source report, positive adjusted equity,
every position notional known, and an unrounded ratio strictly above 3. A stale
known value qualifies; an unknown value does not. One plain-text message per
portfolio lists every affected account without a parent ratio.

The alert repeats after every completed cycle while the condition holds. There
is no deduplication, delivery state, outbox, or recovery message. A send failure
is logged and never changes the cycle result; the next cycle naturally retries.

The bot token is the `TG_RISK_TOKEN` environment secret. The chat id is ordinary
TOML, `[telegram] chat_id`. Telegram has its own explicit `[telegram.proxy]`
policy with the same validation as `collection.proxy`, defaulting to direct; it
never reads proxy environment variables. The frontend warning remains unchanged,
so the rule is implemented in both Go and TypeScript and covered by boundary
tests.

## Consequences

- The owner receives a proactive message even when no browser is open.
- Every cycle is independent, so a persistent breach produces a message every
  cycle (normally every 30 minutes). Restarts and cycle replays can duplicate a
  message; a failed send is simply retried by the next cycle.
- The notification is not a margin call, liquidation prediction, monitoring
  service, or durable delivery record.
- Two implementations of the same predicate can drift. Shared boundary cases and
  the domain rule link both tests; moving the predicate fully server-side
  remains a future option.
- The local deterministic `leverage-warning` seed sends real messages when the
  feature is configured, so development should use a test group or omit the
  token.

## Alternatives

A transactional outbox with a drainer would provide durable at-least-once
delivery, and a state table could suppress repeated reminders. Both add a
migration, retention, and retry machinery that the owner deferred for this first
version. A browser-triggered alert would miss the case where the page is closed.
A server-computed Overview flag would remove the duplicated predicate but would
change the existing API and frontend warning path.
