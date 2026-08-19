# Identity Multiplexer

Identity Multiplexer is a small Go reverse proxy for using more than one active
session with an identity provider (IdP) on the same browser origin.

The proxy keeps the IdP unchanged. It stores the selected IdP session IDs in one
encrypted, authenticated cookie and sends only the selected IdP cookie upstream.

## Project status

This repository contains the first secure core. It is ready for local testing
and adapter work. Test it with the exact IdP version and flow used by your team
before production use.

The core currently supports:

- `authuser=0`, `authuser=1`, and other numeric indexes.
- `authuser=new` for a new login context.
- `X-Auth-User-Index` as a header alternative.
- Per-session logout isolation.
- AES-256-GCM encrypted session state.
- No database in the proxy.
- Health checks and a small account list API.

## Documentation

The public project documentation lives in the [GitHub Wiki](https://github.com/4nass/idmux-proxy/wiki).
It can be edited on GitHub or from the command line.

The `docs/` folder contains technical notes that belong with the source code.

## Security model

- The browser never receives the native IdP session cookie.
- The composite cookie is `Secure`, `HttpOnly`, and `SameSite=Lax` by default.
- The cookie value is encrypted and authenticated with AES-256-GCM.
- The proxy removes client-supplied forwarding and session-routing headers.
- A bad or expired composite cookie is rejected before the request reaches the IdP.
- Account list requests use `Origin` and `Referer` checks when those headers are present.
- IdP `Clear-Site-Data` responses are not allowed to clear every local session.

The proxy is stateless, so an encrypted cookie can be replayed until it expires.
Use a short session lifetime, rotate the encryption key with a planned migration,
and put the proxy behind normal TLS and network controls. A future state backend
can add server-side revocation without changing the HTTP routing model.

## Quick start

Set an upstream IdP URL and a 32-byte base64 key:

```powershell
$env:IDMUX_ENV = "development"
$env:IDMUX_UPSTREAM_URL = "http://127.0.0.1:8081"
$env:IDMUX_COOKIE_NAME = "KEYCLOAK_IDENTITY"
$bytes = [byte[]]::new(32)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
$env:IDMUX_ENCRYPTION_KEY = [Convert]::ToBase64String($bytes)
$env:IDMUX_COOKIE_SECURE = "false"
$env:IDMUX_TRUSTED_ORIGINS = "http://127.0.0.1:9000"
wsl -e bash -lc "cd /home/anass/workspace/idmux && go run ./cmd/idmux-proxy"
```

For production, keep `IDMUX_COOKIE_SECURE=true`, use HTTPS, and load the key from a
secret manager. Never commit the key.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `IDMUX_ENV` | `production` | Use `development` only for local HTTP tests. |
| `IDMUX_LISTEN_PORT` | `:9000` | Local listen address. |
| `IDMUX_UPSTREAM_URL` | required | IdP URL, including scheme and host. |
| `IDMUX_COOKIE_NAME` | `KEYCLOAK_IDENTITY` | Native IdP session cookie name. |
| `IDMUX_SESSION_COOKIE_NAME` | `IDMUX_SESSION` | Encrypted browser cookie name. |
| `IDMUX_ENCRYPTION_KEY` | required | Active 32-byte AES-256 key in base64. |
| `IDMUX_ENCRYPTION_KEY_PREVIOUS` | empty | Previous base64 keys used during rotation. |
| `IDMUX_COOKIE_SECURE` | `true` | Use `Secure` on the composite cookie. False is allowed only in development. |
| `IDMUX_COOKIE_SAMESITE` | `lax` | `lax`, `strict`, or `none`. |
| `IDMUX_SESSION_TTL` | `12h` | Maximum age of encrypted session state. |
| `IDMUX_MAX_SESSIONS` | `8` | Maximum sessions held in one cookie. |
| `IDMUX_LOGOUT_PATH_PREFIX` | `/protocol/openid-connect/logout` | IdP logout path prefix. |
| `IDMUX_TRUSTED_ORIGINS` | empty | Comma-separated origins allowed for the account list API. |

## HTTP behavior

The proxy reads the session index from the `authuser` query parameter. If the
query parameter is absent, it reads `X-Auth-User-Index`. If both are absent,
index `0` is used.

For a selected session, the upstream request contains only:

```http
Cookie: KEYCLOAK_IDENTITY=<selected-idp-session>
```

The proxy removes the internal `IDMUX_SESSION` cookie before forwarding. When
the IdP returns its native session cookie, the proxy stores its value in the
encrypted composite cookie and removes the native `Set-Cookie` header.

Internal endpoints:

- `GET /__idmux/healthz`
- `GET /__idmux/readyz`
- `GET /__idmux/sessions`

The sessions endpoint returns indexes and timestamps only. It never returns the
encrypted IdP session IDs.

## Repository layout

```text
cmd/idmux-proxy/  process entry point
internal/config/            environment configuration
internal/session/           encrypted state and index rules
internal/proxy/             HTTP routing and cookie virtualization
deploy/                     container image
.github/                    contribution checks
```

## GitOps

Git is the source of truth for code, validation, releases, and deployment inputs.
Read [`docs/gitops.md`](docs/gitops.md) for the branch, release, secret, and
rollback rules.

## Contribution

Keep the core small and provider-neutral. New IdP behavior should be added as a
tested adapter or a focused option, not as a provider-specific branch in the
cookie engine. Read `CONTRIBUTING.md` and `SECURITY.md` before opening a change.
