---
title: Security invariants
---

# Security invariants

These rules must remain true under normal traffic, malformed input, upstream
errors, key rotation, and concurrent requests.

## Cryptography and cookies

- `IDMUX_SESSION` never contains a clear IdP session ID.
- AES-256-GCM authenticates and encrypts the complete client state.
- Every encryption uses a fresh random nonce.
- The encryption key is never logged, returned in a header, or exposed in a metric.
- Every proxy-issued session cookie is `HttpOnly`, `Secure`, `SameSite=Lax` or
  `Strict`, and `Path=/`.
- Previous encryption keys may decrypt old cookies during planned rotation.

## Routing and isolation

- An upstream request contains zero or one configured IdP identity cookie.
- `IDMUX_SESSION` is never forwarded to the IdP.
- `authuser=new` sends no IdP identity cookie upstream.
- Missing or invalid routing input follows one deterministic rule: use session
  `0` when it exists, otherwise start a fresh session context.
- A captured `Set-Cookie` changes only the selected session slot.
- Logout removes only the selected slot and keeps other stable indexes unchanged.
- Repeated routing query parameters or headers are rejected.
- The proxy sends one canonical `authuser` value upstream.

## Runtime safety

- No user request data is stored in global or package-level mutable state.
- Upstream failures return a generic response with `Cache-Control: no-store`.
- Raw upstream error details are not sent to the client or written to logs.
- Control routes reject multiple `Origin` or `Referer` headers.
- Invalid input returns a safe HTTP response and never causes a panic.
- Routing, proxy, and internal headers are removed before they can cross a
  trust boundary.
- Logs use JSON and never contain cookie values, encryption keys, or IdP session IDs.

## Required tests

The test suite must cover:

1. a new login with zero IdP cookies upstream;
2. encrypted cookie creation after an IdP `Set-Cookie`;
3. isolated routing for a non-default index;
4. deterministic default routing;
5. corrupted and expired cookies;
6. key rotation;
7. stable indexes after targeted logout;
8. malformed and oversized input without panic;
9. repeated or conflicting routing input;
10. malformed upstream identity cookies and safe error responses;
11. repeated `Origin` or `Referer` headers on control routes.
