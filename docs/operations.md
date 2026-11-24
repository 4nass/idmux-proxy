---
title: Operations
---

# Operations

## Production requirements

- Run the public browser connection and IdP upstream over TLS. If an ingress
  terminates TLS, restrict the internal ingress-to-proxy hop; use mesh mTLS when
  that hop must also be encrypted. IdMux does not serve TLS itself.
- Keep `IDMUX_COOKIE_SECURE=true`.
- Store the key in a secret manager and mount it with `IDMUX_ENCRYPTION_KEY_FILE`.
- Use a short `IDMUX_SESSION_TTL`.
- Set `IDMUX_MAX_REQUEST_BODY_BYTES` to the smallest size that fits the IdP flow.
- Set `IDMUX_UPSTREAM_TIMEOUT` to a value that fits the IdP flow and stays below
  the deployment's response timeout.
- Restrict network access to the proxy and upstream.
- Set `IDMUX_TRUSTED_ORIGINS` for the public browser origin.
- Test login, callback, token refresh, account switching, and logout with the
  exact IdP version used in production.

See the [deployment guide](deployment.md) for Docker and Kubernetes examples,
image pinning, probes, and network policy notes.

## Key rotation

Set the new active key in `IDMUX_ENCRYPTION_KEY` or its mounted file and keep
the old key in `IDMUX_ENCRYPTION_KEY_PREVIOUS` during the migration window. Do
not set both active key sources. Deploy all replicas with the same key list.
Remove the old key only after all old cookies have expired according to
`IDMUX_SESSION_TTL`.

## Logs

Logs are JSON. They may contain a path, event name, status, or selected numeric
index. They must never contain cookie values, tokens, encryption keys, or user
session IDs.
