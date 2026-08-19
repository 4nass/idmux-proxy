---
title: Contributing
---

# Contributing

Thank you for helping with IdMux. Small, clear changes are easier to review
and safer to run.

## Before you start

Read the [security rules](security-invariants.html) and
[SECURITY.md](https://github.com/4nass/idmux-proxy/blob/main/SECURITY.md).

## Make a change

1. Fork the repository or create a short-lived branch from `main`.
2. Make one focused change.
3. Add or update tests.
4. Run `gofmt`, `go test -race ./...`, `go vet ./...`, and `golangci-lint run`.
5. Open a pull request with a short description and test results.

Keep provider-specific behavior out of the core cookie engine. Do not add
password, token, JWT, or business logic to the proxy.

## Good pull requests

- Explain what changed and why.
- Explain the security impact.
- Show the checks that passed.
- Keep the commit history small and clear.
- Do not include secrets, real cookies, tokens, or customer data.

The code owner reviews changes before they enter `main`.
