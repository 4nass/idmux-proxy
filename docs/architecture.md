---
title: Architecture
---

# Architecture

## Goal

IdMux Proxy virtualizes one IdP session cookie into several isolated browser
sessions. The IdP remains the authority for login, tokens, passwords, and
business rules.

## Request flow

```text
Browser
  -> IdMux Proxy
     -> read and authenticate IDMUX_SESSION
     -> select authuser index
     -> remove proxy cookies and client routing headers
     -> send zero or one configured IdP identity cookie
  -> IdP
     -> return response
  -> IdMux Proxy
     -> capture the configured IdP Set-Cookie
     -> update only the selected session slot
     -> send the encrypted IDMUX_SESSION cookie
```

## Stateless rule

The proxy has no database, Redis, shared memory state, or user session cache.
All session selection state is inside the authenticated encrypted cookie. This
allows several proxy replicas to run behind a load balancer.

The proxy keeps only immutable process configuration and initialized crypto
objects in memory. Per-request data lives in the request context.

## Responsibility boundary

The proxy only handles:

- HTTP cookie and header rewriting;
- session index selection;
- encrypted session state;
- response cookie capture;
- targeted logout routing.

The proxy never:

- reads or validates JWT claims;
- validates passwords;
- changes OIDC or SAML meaning;
- makes authorization decisions;
- owns user profiles or business data.

## Package layout

`internal/` is intentional. These packages are implementation details, not a
public library API. New provider-specific behavior should be isolated behind a
small adapter or configuration option and covered by integration tests.
