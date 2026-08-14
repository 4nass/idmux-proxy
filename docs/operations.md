# Operations

## Production requirements

- Run the proxy and IdP over TLS.
- Keep `IDMUX_COOKIE_SECURE=true`.
- Store `IDMUX_ENCRYPTION_KEY` in a secret manager.
- Use a short `IDMUX_SESSION_TTL`.
- Restrict network access to the proxy and upstream.
- Set `IDMUX_TRUSTED_ORIGINS` for the public browser origin.
- Test login, callback, token refresh, account switching, and logout with the
  exact IdP version used in production.

## Key rotation

Set the new active key in `IDMUX_ENCRYPTION_KEY` and keep the old key in
`IDMUX_ENCRYPTION_KEY_PREVIOUS` during the migration window. Deploy all replicas
with the same key list. Remove the old key only after all old cookies have
expired according to `IDMUX_SESSION_TTL`.

## Logs

Logs are JSON. They may contain a path, event name, status, or selected numeric
index. They must never contain cookie values, tokens, encryption keys, or user
session IDs.
