---
status: accepted
---

# Use IPv4 for Platform connections

Servers with both IP families can reach one Platform over IPv4 and another over IPv6. An API key restricted to a server's IPv4 address then fails authentication on the IPv6 request, even though both requests came from the same server.

Default live Adapter HTTP clients dial over IPv4. This includes the Kraken WebSocket connection, which uses the same client. Explicitly supplied clients remain available for fixture replay and other tests. Keep the default transport's TLS, connection reuse, and environment proxy behavior.

This makes one stable IPv4 egress address sufficient for Platform key allowlists without changing the server's network settings. A server without IPv4 connectivity cannot use the default live Adapters. When a proxy is configured, the proxy chooses the address family it uses toward the Platform, so its egress IP must be allowed separately.
