## What changed?

<!-- Explain the user or security problem. -->

## Security impact

<!-- Explain whether cookies, routing, authentication, or trust boundaries changed. -->

## GitOps impact

<!-- Explain changes to images, deployment inputs, environments, or workflows. -->

## Tests

- [ ] `gofmt` check
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] Container build checked when relevant
- [ ] Manual IdP flow tested when relevant

## Review checklist

- [ ] The change is small and focused.
- [ ] No secrets, real cookies, tokens, or customer data are included.
- [ ] Documentation and rollback notes are updated when needed.
