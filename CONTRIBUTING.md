# Contributing

Thank you for helping improve Identity Multiplexer.

## Basic rules

- Use simple English in code, comments, commits, and documentation.
- Keep changes small and focused.
- Do not add provider secrets, real cookies, or customer data to tests.
- Add tests for security or routing behavior.
- Use the standard library unless a dependency has a clear security and
  maintenance benefit.
- Keep the proxy provider-neutral and secure by default.

## Local checks

Run these commands from the project directory:

```text
test -z "$(gofmt -l ./cmd ./internal)"
go test -race ./...
go vet ./...
go build -trimpath -o /tmp/idmux-proxy ./cmd/idmux-proxy
docker build --file deploy/Dockerfile --tag idmux-proxy:local .
```

See [`docs/gitops.md`](docs/gitops.md) for the branch, review, release, and
deployment workflow.

## Pull requests

Explain the problem, the security impact, and how the change was tested. Keep
the pull request focused on one idea. Do not include personal attribution in
generated files or release metadata. Changes to deployment inputs must use
immutable image digests and include rollback notes.
