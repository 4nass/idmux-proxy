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
| `IDMUX_LOGIN_START_PATH` | `/realms/my-realm/protocol/openid-connect/auth` |
| `IDMUX_FRESH_LOGIN_PATH` | `/realms/my-realm/login-actions/authenticate` |
| `IDMUX_LOGOUT_PATH_PREFIX` | `/realms/my-realm/protocol/openid-connect/logout` |

Start a new account at the authorization endpoint with `authuser=new`. Use
`authuser=0` and `authuser=1` on later authorization requests to select an
account. Keycloak does not copy the selector into its login form action.
A form POST with no selector starts a new slot. An explicit index selects an
existing slot. Invalid or conflicting selectors receive HTTP 400.

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

The case starts a second login while the first form is open and checks that
closing an unfinished tab releases the next one. It checks both account
callbacks, silent sign-in for both slots, code
exchange, token refresh, UserInfo, switching to the second proxy after the
first stops, and logout for only one account. It also checks that Keycloak
rejects token refresh for the logged-out account.

## Limits of this profile

With both paths configured, a top-level browser request for a new account in
code flow with an HTTP or HTTPS query callback opens a waiting page. Use HTTPS
for the callback in production. The page holds a
[Web Lock](https://www.w3.org/TR/web-locks/) on the proxy origin while the
user signs in inside a same-origin frame. The second tab opens its form
automatically after the first callback or when the first tab closes. The
client receives the redirect in the top-level tab. Each queued authorization
starts with an empty upstream Cookie header so Keycloak creates a new login
transaction even after a tab was closed. Subsequent requests carry the new
transaction cookies. The IdP must allow its
login page to be framed by the same origin.

Use HTTPS (or loopback for tests) and a browser with JavaScript, Fetch Metadata
and Web Locks. The queue coordinates interactive logins in this browser on
this origin. Other concurrent operations can still write the same composite
cookie. Clear `IDMUX_LOGIN_START_PATH` on all replicas to turn off the queue;
finish or close any open login forms before changing this setting.

This profile covers the password form and logout with an ID token hint.
Keycloak's login status iframe, other authentication steps, SAML, and logout
confirmation pages need their own integration coverage.
