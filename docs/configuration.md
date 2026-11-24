---
title: Configuration
---

# Configuration

IdMux reads its settings from environment variables. Set either
`IDMUX_ENCRYPTION_KEY` or `IDMUX_ENCRYPTION_KEY_FILE`, not both. The key value
must decode to exactly 32 bytes.

| Variable | Default | Meaning |
| --- | --- | --- |
| `IDMUX_ENV` | `production` | Use `development` only for local HTTP tests. |
| `IDMUX_LISTEN_PORT` | `:9000` | Address where IdMux listens. |
| `IDMUX_UPSTREAM_URL` | required | IdP URL. HTTPS is required in production; HTTP is for development only. |
| `IDMUX_COOKIE_NAME` | `KEYCLOAK_IDENTITY` | Native IdP session cookie name. |
| `IDMUX_SESSION_COOKIE_NAME` | `IDMUX_SESSION` | Encrypted browser cookie name. |
| `IDMUX_ENCRYPTION_KEY` | one required | Active AES-256 key in base64. Use the file option in production. |
| `IDMUX_ENCRYPTION_KEY_FILE` | empty | File containing the active base64 key. Use this with a mounted secret. |
| `IDMUX_ENCRYPTION_KEY_PREVIOUS` | empty | Old base64 keys during rotation. |
| `IDMUX_COOKIE_SECURE` | `true` | Use `Secure` on the browser cookie. |
| `IDMUX_COOKIE_SAMESITE` | `lax` | `lax`, `strict`, or `none`. |
| `IDMUX_SESSION_TTL` | `12h` | Maximum age of session state. |
| `IDMUX_MAX_SESSIONS` | `8` | Maximum number of session slots, from 1 to 32. |
| `IDMUX_MAX_REQUEST_BODY_BYTES` | `1048576` | Maximum request body size, from 1 byte to 1 GiB. |
| `IDMUX_UPSTREAM_TIMEOUT` | `45s` | Maximum time for one request to the IdP, from 1 second to 5 minutes. |
| `IDMUX_LOGOUT_PATH_PREFIX` | IdP logout path | Path used for targeted logout. |
| `IDMUX_CONTROL_PATH_PREFIX` | `/__idmux` | Path for health, sessions, and metrics endpoints. |
| `IDMUX_TRUSTED_ORIGINS` | empty | Comma-separated browser origins for the session list. |

## Safe defaults

Keep `IDMUX_COOKIE_SECURE=true` in production. Use HTTPS from the browser to the
public ingress and from IdMux to the IdP. IdMux does not serve TLS itself. If
the ingress sends HTTP to IdMux, restrict that internal hop; use mesh mTLS when
it must also be encrypted. Keep the active key in a secret manager. In
production, use `IDMUX_ENCRYPTION_KEY_FILE` to read it from a mounted secret.
Never put a key, token, cookie, or real user data in Git.

The proxy returns `413 Request Entity Too Large` when a request body exceeds
this limit. Choose a limit that fits the IdP's login and protocol forms. See the
[deployment guide](deployment.md) for container and Kubernetes examples.

`IDMUX_COOKIE_SECURE=false` is accepted only when `IDMUX_ENV=development`.
`IDMUX_COOKIE_SAMESITE=none` also needs `IDMUX_COOKIE_SECURE=true`.

The metrics endpoint is `GET /__idmux/metrics` by default. Restrict it at
the network or deployment gateway if it should only be available to scrapers.
See the [observability guide](observability.md) for safe log fields and metric
names.

## Key rotation

Put the new key in `IDMUX_ENCRYPTION_KEY` or its mounted file, and put the old
key in `IDMUX_ENCRYPTION_KEY_PREVIOUS`. Set only one active key source. Run all
replicas with the same key list. Remove the old key after all old cookies have
expired.
