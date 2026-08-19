---
layout: default
title: IdMux Proxy
---

IdMux Proxy is a small Go sidecar proxy. It lets one browser use more than one
active session with the same identity provider (IdP).

This page gives a short overview of the project.

## Project context

### Objectives

- Support more than one active IdP session on one browser origin.
- Keep the existing IdP unchanged.
- Keep the proxy small, provider-neutral, and easy to operate.
- Use zero-trust rules and secure defaults.

### Scope

IdMux sits in front of the IdP. It virtualizes the IdP session cookie at the
HTTP layer. It selects a session, sends only that session upstream, and stores
the session map in an encrypted browser cookie.

IdMux does not replace the IdP. It does not validate passwords, change OIDC or
SAML meaning, own user profiles, or make business authorization decisions.

### Constraints

- The IdP must remain unchanged.
- The proxy must work with common OIDC and SAML IdPs.
- The first version must be stateless and easy to run as a sidecar.
- Secrets, cookies, tokens, and session IDs must not appear in logs or Git.
- The project must stay open to review and contribution.

## Architectural principles

- **Sidecar first:** keep the complex IAM core outside the proxy.
- **HTTP virtualization:** rewrite only the cookies and headers needed for
  session selection.
- **IdP authority:** let the IdP own login, tokens, logout, and user data.
- **Zero trust:** treat client headers and cookie input as untrusted.
- **Secure by design:** encrypt and authenticate the session state, use secure
  cookie flags, and remove internal headers at the trust boundary.
- **Provider neutral:** avoid provider-specific logic in the core proxy.

## Technical constraints

### Performance

- Keep request handling in memory.
- Avoid a database or network call for normal session selection.
- Keep the proxy image small and the startup path short.
- Do not add work that is not needed for the current request.

### Security

- Use AES-256-GCM with a fresh nonce for every encrypted cookie.
- Reject invalid, expired, or oversized session state before proxying.
- Forward zero or one selected IdP cookie, never the composite cookie.
- Use `HttpOnly`, `Secure`, and `SameSite` cookie settings in production.
- Support planned encryption key rotation.
- Test malformed input, logout isolation, and concurrent requests.

### Scalability

- Keep replicas stateless so they can run behind a load balancer.
- Keep the maximum session count bounded.
- Use an external state backend only when server-side revocation is needed.
- Deploy immutable container image digests.

## DevOps & Operations

### Git

- Use short-lived branches from `main`.
- Keep commits small and easy to review.
- Use pull requests for code and security changes.

### CI/CD

- Run tests, lint, CodeQL, vulnerability checks, and image scans.
- Build release images from version tags.
- Publish SBOM and provenance data.
- Deploy immutable image digests, never `latest`.

### Monitoring

- Check `/__idmux/healthz` for process health.
- Check `/__idmux/readyz` before sending traffic.
- Alert on failed checks, upstream errors, and unusual logout or session errors.

### Logs

- Use JSON logs with useful event names and status values.
- Never log cookies, tokens, keys, or IdP session IDs.
- Keep log retention short and protect access to production logs.

## Learn more

- [Native GitHub Wiki](https://github.com/4nass/idmux-proxy/wiki)
- [Source repository](https://github.com/4nass/idmux-proxy)
