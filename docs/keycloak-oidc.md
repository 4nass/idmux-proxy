---
title: Keycloak OIDC
---

# Keycloak OIDC

This profile was tested with Keycloak 26.8.2, the standard browser login,
and OIDC authorization code flow with PKCE. The test uses two accounts in
one Chromium session.

## Proxy settings

Set the full public URL in Keycloak's [hostname settings](https://www.keycloak.org/server/hostname)
so form actions and OIDC discovery use the URL of the proxy. Keep the same
IdMux cookie settings and encryption keys on both replicas.

For a realm called `my-realm`, set:

| Setting | Value |
| --- | --- |
| `IDMUX_COOKIE_NAME` | `KEYCLOAK_IDENTITY` |
| `IDMUX_FRESH_LOGIN_PATH` | `/realms/my-realm/login-actions/authenticate` |
| `IDMUX_LOGOUT_PATH_PREFIX` | `/realms/my-realm/protocol/openid-connect/logout` |

Start a new account at the authorization endpoint with `authuser=new`. Use
`authuser=0` and `authuser=1` on later authorization requests to select an
account. Keycloak does not copy the selector into its login form action.
IdMux treats a POST to the configured fresh login path as a new login, so a
successful form submission gets a new slot.

For browser logout in this profile, send the slot index and the matching
`id_token_hint` in the RP-initiated logout request. The client is responsible
for using the index and ID token from the same login.

## Cookie handling

The test records cookie names from response headers. These are the cookies
in the tested username and password flow.

| Cookie | Role in the flow |
| --- | --- |
| `AUTH_SESSION_ID` | Tracks the login transaction across the Keycloak form and its submission. IdMux passes it through. |
| `KC_AUTH_SESSION_HASH` | Lets the Keycloak login pages track the current authentication session. IdMux passes it through. |
| `KC_RESTART` | Lets Keycloak restart an interrupted login. IdMux passes it through. |
| `KEYCLOAK_IDENTITY` | The SSO credential. IdMux captures it in the selected slot and sends it upstream only for that slot. |
| `KEYCLOAK_SESSION` | The browser-facing session status cookie. IdMux passes it through. It remains shared and is outside this profile's session isolation. |
| `IDMUX_SESSION` | IdMux's encrypted session map. The browser sends it to IdMux and both replicas can read it. |

Code exchange, refresh, and UserInfo use the code, refresh token, or access
token, respectively. Those requests need no browser cookies in this profile.

## Run the browser test

Use Go, Docker Compose, Bash, and Node.js 24. From the repo root:

```sh
npm --prefix integration/keycloak ci
npm --prefix integration/keycloak exec -- playwright install chromium
bash integration/keycloak/run.sh
```

On Linux, install Chromium's system libraries with
`npm --prefix integration/keycloak exec -- playwright install --with-deps chromium`
if they are missing. The test uses local ports 19000 through 19003 and 19080.
It starts a pinned Keycloak container with a test realm and two local IdMux
processes, then cleans up that Compose project when it ends. The realm and
its example credentials are only for this loopback test fixture.

The case checks both account callbacks, silent sign-in for both slots, code
exchange, token refresh, UserInfo, switching to the second proxy after the
first stops, and logout for only one account. It also checks that Keycloak
rejects token refresh for the logged-out account.

## Limits of this profile

Both sessions can stay active once signed in. Open each login form after
the previous login completes. In an overlapping attempt on 26.8.2, Keycloak
returned the second tab to its login action instead of completing the OIDC
callback. The in-progress authentication cookies are shared by the tabs.
IdMux also cannot merge writes made at the same time from the same stateless
cookie.

This profile covers the password form and logout with an ID token hint.
Keycloak's login status iframe, other authentication steps, SAML, and logout
confirmation pages need their own integration coverage.
