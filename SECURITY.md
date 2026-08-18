# Security policy

Please do not report a suspected vulnerability in a public issue.

Send a private report to the maintainers with:

- a short description;
- affected version or commit;
- clear reproduction steps;
- possible impact;
- a suggested fix, if available.

Do not include real user tokens, cookies, keys, or personal data.

The most important deployment rules are:

- use HTTPS between the browser, proxy, and IdP;
- keep `IDMUX_COOKIE_SECURE=true`;
- store `IDMUX_ENCRYPTION_KEY` in a secret manager;
- set `IDMUX_ENCRYPTION_KEY_PREVIOUS` only during key rotation;
- use a short `IDMUX_SESSION_TTL`;
- restrict access to the proxy and upstream network;
- verify logout, refresh, callback, and parallel login flows with your IdP.
