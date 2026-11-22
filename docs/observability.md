---
title: Observability
---

# Observability

IdMux writes JSON logs to standard output. It keeps counters and latency
histograms in memory. Read them at `GET /__idmux/metrics`. The endpoint has
no user, cookie, key, path, or query labels. Scrape each replica separately.
Metrics reset when a process restarts.

## Request IDs

IdMux makes a new random 128-bit ID for every HTTP request. It replaces any
client-supplied `X-Request-ID`, sends the safe value to the IdP, and returns
it to the browser. Use this value to match a request log to an IdP-side log.
Do not treat it as authentication or pass it to a user-facing URL.

## Logs

Completed requests contain a safe path category, method, status, duration in
milliseconds, and request ID. A numeric session index is included only after
the proxy has selected it. Unknown methods are written as `OTHER`. Arbitrary
request paths are written as `/`; the query string is never logged.

Example with synthetic values:

```json
{"time":"2026-01-01T12:00:00Z","level":"INFO","msg":"request completed","event":"request_completed","request_id":"8b37dfdd7434264f5b96867590d81df2","method":"GET","path":"/","status":200,"duration_ms":14,"session_index":1}
```

Never log cookie values, tokens, keys, user IDs, raw URLs, query strings,
request or response bodies, or upstream error text. Upstream failure logs
contain the request ID and safe path category only.

## Metrics

The endpoint exposes these counters:

- `idmux_requests_total`
- `idmux_upstream_requests_total`
- `idmux_upstream_errors_total`
- `idmux_fallback_total`
- `idmux_invalid_cookies_total`
- `idmux_logout_total`

It also exposes `idmux_request_duration_seconds` and
`idmux_upstream_duration_seconds` histograms. An upstream error is a network
or response-processing failure, or an upstream HTTP status of 500 or higher.
Logout counts requests to the configured logout path. Counters have no labels,
so user data cannot become a metric label.

Restrict access to the metrics endpoint with network policy or the deployment
gateway if metrics should be private. Metrics contain aggregate service health,
not session state. Do not add user IDs, cookie values, tokens, arbitrary paths,
or request IDs as metric labels.
