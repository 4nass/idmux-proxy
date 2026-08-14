# Contributing

Thank you for helping improve Identity Multiplexer.

## Basic rules

- Use simple English in code, comments, commits, and documentation.
- Keep changes small and focused.
- Do not add provider secrets, real cookies, or customer data to tests.
- Add tests for security or routing behavior.
- Use the standard library unless a dependency has a clear security and
  maintenance benefit.

## Local checks

Run these commands from the project directory:

```text
go test ./...
go vet ./...
gofmt -w ./cmd ./internal
```

## Pull requests

Explain the problem, the security impact, and how the change was tested. Keep
the pull request focused on one idea. Do not include personal attribution in
generated files or release metadata.
