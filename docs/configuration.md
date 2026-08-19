---
title: Configuration
---

# Configuration

IdMux reads its settings from environment variables. The encryption key must
decode to exactly 32 bytes.

| Variable | Default | Meaning |
| --- | --- | --- |
| `IDMUX_ENV` | `production` | Use `development` only for local HTTP tests. |
| `IDMUX_LISTEN_PORT` | `:9000` | Address where IdMux listens. |
| `IDMUX_UPSTREAM_URL` | required | IdP URL, with `http` or `https`. |
| `IDMUX_COOKIE_NAME` | `KEYCLOAK_IDENTITY` | Native IdP session cookie name. |
| `IDMUX_SESSION_COOKIE_NAME` | `IDMUX_SESSION` | Encrypted browser cookie name. |
| `IDMUX_ENCRYPTION_KEY` | required | Active AES-256 key in base64. |
| `IDMUX_ENCRYPTION_KEY_PREVIOUS` | empty | Old base64 keys during rotation. |
| `IDMUX_COOKIE_SECURE` | `true` | Use `Secure` on the browser cookie. |
| `IDMUX_COOKIE_SAMESITE` | `lax` | `lax`, `strict`, or `none`. |
| `IDMUX_SESSION_TTL` | `12h` | Maximum age of session state. |
| `IDMUX_MAX_SESSIONS` | `8` | Maximum number of session slots, from 1 to 32. |
| `IDMUX_LOGOUT_PATH_PREFIX` | IdP logout path | Path used for targeted logout. |
| `IDMUX_CONTROL_PATH_PREFIX` | `/__idmux` | Path for health and session endpoints. |
| `IDMUX_TRUSTED_ORIGINS` | empty | Comma-separated browser origins for the session list. |

## Safe defaults

Keep `IDMUX_COOKIE_SECURE=true` in production. Use HTTPS between the browser,
IdMux, and the IdP. Keep the active key in a secret manager. Never put a key,
token, cookie, or real user data in Git.

`IDMUX_COOKIE_SECURE=false` is accepted only when `IDMUX_ENV=development`.
`IDMUX_COOKIE_SAMESITE=none` also needs `IDMUX_COOKIE_SECURE=true`.

## Key rotation

Put the new key in `IDMUX_ENCRYPTION_KEY` and the old key in
`IDMUX_ENCRYPTION_KEY_PREVIOUS`. Run all replicas with the same key list.
Remove the old key after all old cookies have expired.
