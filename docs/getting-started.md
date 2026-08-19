---
title: Getting started
---

# Getting started

## What you need

- Go 1.26 or newer.
- An IdP that is already running.
- A 32-byte encryption key, encoded with base64.

## Run locally

From the repository root, set a few variables:

```powershell
$env:IDMUX_ENV = "development"
$env:IDMUX_LISTEN_PORT = ":9000"
$env:IDMUX_UPSTREAM_URL = "http://127.0.0.1:8081"
$env:IDMUX_COOKIE_NAME = "KEYCLOAK_IDENTITY"
$env:IDMUX_COOKIE_SECURE = "false"
$env:IDMUX_TRUSTED_ORIGINS = "http://127.0.0.1:9000"
$bytes = [byte[]]::new(32)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
$env:IDMUX_ENCRYPTION_KEY = [Convert]::ToBase64String($bytes)
```

Then start the proxy:

```text
go run ./cmd/idmux-proxy
```

Open the public proxy address, not the IdP address. In this example it is
`http://127.0.0.1:9000`.

## Select a session

Use `authuser=0` or `authuser=1` in the URL. You can also send the
`X-Auth-User-Index` header. If no index is given, IdMux uses session `0`.

To start a new login context, use `authuser=new`.

## Check the proxy

The proxy has these local endpoints:

- `GET /__idmux/healthz`
- `GET /__idmux/readyz`
- `GET /__idmux/sessions`

The sessions endpoint returns indexes and timestamps. It never returns the
encrypted IdP session values.

## Next step

Read [Configuration](configuration.html) before using the proxy with a real
IdP.
