# Development

## Local checks

```text
gofmt -w ./cmd ./internal
go test -race ./...
go vet ./...
golangci-lint run --timeout=5m
```

The repository uses the version 2 format in `.golangci.yml`. The configuration
enables security checks through `gosec` and keeps the default static checks.

## Local configuration

Copy `.env.example`, create a random 32-byte base64 key, and set the upstream
IdP URL. Use `IDMUX_ENV=development` only when testing over plain HTTP.

## Contribution rule

Keep the proxy provider-neutral. A new IdP integration must not add password,
token, JWT, or business logic to the core proxy. Add a focused test for every
new routing or cookie rule.

## Local containers

Set a pinned Keycloak image and a generated base64 key before starting the
example stack:

```text
KEYCLOAK_IMAGE=quay.io/keycloak/keycloak:<pinned-version>
IDMUX_ENCRYPTION_KEY=<32-byte-base64-key>
docker compose -f deploy/docker-compose.example.yml up --build
```

After building the local image, scan it before using it:

```text
trivy image --severity HIGH,CRITICAL --ignore-unfixed idmux-proxy:local
```
