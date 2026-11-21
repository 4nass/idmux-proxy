import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { randomBytes, createHash } from 'node:crypto';
import { once } from 'node:events';
import { createServer, request } from 'node:http';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from 'playwright';

const edge = 'http://127.0.0.1:19000';
const upstream = 'http://127.0.0.1:19080';
const client = 'http://localhost:19003';
const realm = '/realms/idmux-test';
const oidc = realm + '/protocol/openid-connect';
const clientID = 'idmux-test';
const replicaPorts = [19001, 19002];

async function waitFor(url, name, proxy) {
  for (let attempt = 0; attempt < 90; attempt++) {
    if (proxy && !proxy.isRunning()) throw new Error(name + ' exited during startup');
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(1000) });
      await response.body?.cancel();
      if (response.ok) return;
    } catch {
      // The service may still be starting.
    }
    await delay(1000);
  }
  throw new Error(name + ' did not become ready');
}

function startProxy(binary, port, key) {
  const proxy = spawn(binary, {
    env: {
      IDMUX_ENV: 'development',
      IDMUX_LISTEN_PORT: '127.0.0.1:' + port,
      IDMUX_UPSTREAM_URL: upstream,
      IDMUX_COOKIE_NAME: 'KEYCLOAK_IDENTITY',
      IDMUX_ENCRYPTION_KEY: key,
      IDMUX_COOKIE_SECURE: 'false',
      IDMUX_MAX_SESSIONS: '2',
      IDMUX_LOGOUT_PATH_PREFIX: oidc + '/logout',
      IDMUX_LOGIN_START_PATH: oidc + '/auth',
      IDMUX_FRESH_LOGIN_PATH: realm + '/login-actions/authenticate',
      IDMUX_TRUSTED_ORIGINS: edge,
    },
    stdio: ['ignore', 'ignore', 'inherit'],
  });
  let running = true;
  const exited = new Promise(resolve => {
    const onExit = () => {
      running = false;
      resolve();
    };
    proxy.once('exit', onExit);
    proxy.once('error', onExit);
  });
  return {
    port,
    isRunning: () => running,
    stop: async () => {
      if (running) proxy.kill('SIGTERM');
      await exited;
    },
  };
}

async function startServer(port, handler) {
  const server = createServer(handler);
  server.listen(port, '127.0.0.1');
  await once(server, 'listening');
  return server;
}

function forward(req, res, port, cookieNames) {
  const outbound = request({
    host: '127.0.0.1', port, path: req.url, method: req.method, headers: req.headers,
  }, incoming => {
    if (cookieNames) {
      for (const cookie of incoming.headers['set-cookie'] ?? []) cookieNames.add(cookie.split('=', 1)[0]);
    }
    res.writeHead(incoming.statusCode, incoming.headers);
    incoming.pipe(res);
  });
  outbound.on('error', () => {
    if (res.headersSent) res.destroy();
    else res.writeHead(502).end();
  });
  res.on('close', () => outbound.destroy());
  req.pipe(outbound);
}

async function getJSON(response, name) {
  assert.equal(response.status, 200, name + ' returned an unexpected status');
  return response.json();
}

function tokenResponse(grant) {
  return fetch(edge + oidc + '/token', {
    method: 'POST',
    redirect: 'error',
    body: new URLSearchParams({ client_id: clientID, ...grant }),
    signal: AbortSignal.timeout(10000),
  });
}

async function token(grant) {
  return getJSON(await tokenResponse(grant), grant.grant_type);
}

async function userinfo(accessToken) {
  return getJSON(await fetch(edge + oidc + '/userinfo', {
    redirect: 'error',
    headers: { Authorization: 'Bearer ' + accessToken },
    signal: AbortSignal.timeout(10000),
  }), 'userinfo');
}

function createClient() {
  const pending = new Map();
  const results = new Map();

  async function handle(req, res) {
    const url = new URL(req.url, client);
    if (url.pathname === '/signed-out') {
      res.end('Signed out');
      return;
    }
    if (url.pathname !== '/callback') {
      res.writeHead(404).end();
      return;
    }
    const state = url.searchParams.get('state');
    const verifier = pending.get(state);
    pending.delete(state);
    assert.ok(verifier, 'callback state did not match');
    assert.equal(url.searchParams.get('iss'), edge + realm, 'callback issuer did not match');
    const error = url.searchParams.get('error');
    if (error === 'login_required') {
      results.set(state, { loginRequired: true });
      res.end('Login required');
      return;
    }
    assert.equal(error, null, 'authorization failed');
    const code = url.searchParams.get('code');
    assert.ok(code, 'callback did not contain a code');
    const tokens = await token({
      grant_type: 'authorization_code',
      code,
      code_verifier: verifier,
      redirect_uri: client + '/callback',
    });
    const profile = await userinfo(tokens.access_token);
    results.set(state, { profile, tokens });
    res.end('Signed in');
  }

  return {
    handle: (req, res) => {
      handle(req, res).catch(() => {
        res.writeHead(500).end('Callback failed');
      });
    },
    begin: (target, prompt) => {
      const state = randomBytes(24).toString('base64url');
      const verifier = randomBytes(32).toString('base64url');
      pending.set(state, verifier);
      const address = new URL(edge + oidc + '/auth');
      address.search = new URLSearchParams({
        authuser: target,
        client_id: clientID,
        redirect_uri: client + '/callback',
        response_type: 'code',
        scope: 'openid profile',
        state,
        code_challenge: createHash('sha256').update(verifier).digest('base64url'),
        code_challenge_method: 'S256',
        ...(prompt ? { prompt } : {}),
      });
      return { address: address.href, state };
    },
    finish: async (page, flow) => {
      try {
        await page.waitForURL(url => url.origin === client && url.pathname === '/callback' && url.searchParams.get('state') === flow.state, {
          waitUntil: 'load',
        });
      } catch {
        throw new Error('Browser did not reach the test client callback');
      }
      const result = results.get(flow.state);
      results.delete(flow.state);
      assert.ok(result, 'test client did not complete the callback');
      return result;
    },
  };
}

async function submitLogin(tab, user) {
  const page = tab.frameLocator('iframe');
  await page.locator('#username').fill(user);
  await page.locator('#password').fill('idmux-dev-only');
  await page.locator('#kc-login').click();
}

async function slots(context) {
  const result = await context.request.get(edge + '/__idmux/sessions');
  assert.equal(result.status(), 200, 'session listing failed');
  return (await result.json()).sessions.map(entry => entry.index);
}

async function silentLogin(page, rp, target) {
  const flow = rp.begin(target, 'none');
  await page.goto(flow.address);
  return rp.finish(page, flow);
}

async function refresh(result) {
  return token({ grant_type: 'refresh_token', refresh_token: result.tokens.refresh_token });
}

async function logOut(page, idToken) {
  const logout = new URL(edge + oidc + '/logout');
  const state = randomBytes(24).toString('base64url');
  logout.search = new URLSearchParams({
    authuser: '0',
    id_token_hint: idToken,
    post_logout_redirect_uri: client + '/signed-out',
    state,
  });
  try {
    await page.goto(logout.href);
  } catch {
    throw new Error('Browser logout navigation failed');
  }
  const returned = new URL(page.url());
  assert.ok(returned.origin === client && returned.pathname === '/signed-out', 'logout callback did not arrive');
  assert.ok(returned.searchParams.get('state') === state, 'logout state did not match');
}

async function run(binary) {
  const proxies = [];
  const servers = [];
  let browser;
  try {
    const key = randomBytes(32).toString('base64');
    for (const port of replicaPorts) proxies.push(startProxy(binary, port, key));
    const cookieNames = new Set();
    let requests = 0;
    const usedReplicas = new Set();
    servers.push(await startServer(19000, (req, res) => {
      const live = proxies.filter(proxy => proxy.isRunning());
      if (!live.length) return res.writeHead(502).end();
      const replica = live[requests++ % live.length];
      usedReplicas.add(replica.port);
      forward(req, res, replica.port, cookieNames);
    }));
    const rp = createClient();
    servers.push(await startServer(19003, rp.handle));

    await Promise.all([
      waitFor(upstream + realm + '/.well-known/openid-configuration', 'Keycloak'),
      ...proxies.map(proxy => waitFor('http://127.0.0.1:' + proxy.port + '/__idmux/readyz', 'IdMux replica', proxy)),
    ]);
    browser = await chromium.launch();
    const context = await browser.newContext();
    context.setDefaultTimeout(15000);
    const alicePage = await context.newPage();
    const bobPage = await context.newPage();
    const alice = rp.begin('new');
    const bob = rp.begin('new');
    const abandoned = await context.newPage();
    await abandoned.goto(rp.begin('new').address);
    await abandoned.frameLocator('iframe').locator('#username').waitFor();
    await alicePage.goto(alice.address);
    await alicePage.getByRole('status').waitFor();
    await abandoned.close();
    await alicePage.frameLocator('iframe').locator('#username').waitFor();
    await bobPage.goto(bob.address);
    await bobPage.getByRole('status').waitFor();
    await bobPage.waitForFunction(async () => {
      const locks = await navigator.locks.query();
      return locks.pending.some(lock => lock.name === 'idmux:interactive-login');
    });
    assert.equal(await bobPage.locator('iframe').count(), 0);
    await submitLogin(alicePage, 'alice');
    const aliceResult = await rp.finish(alicePage, alice);
    assert.equal(aliceResult.profile.preferred_username, 'alice');
    assert.deepEqual(await slots(context), [0], 'first login did not create slot 0');

    await submitLogin(bobPage, 'bob');
    const bobResult = await rp.finish(bobPage, bob);
    assert.equal(bobResult.profile.preferred_username, 'bob');
    assert.deepEqual(await slots(context), [0, 1], 'second login did not keep both slots');
    assert.ok(cookieNames.has('IDMUX_SESSION'), 'the proxy did not set a composite cookie');

    assert.equal((await silentLogin(alicePage, rp, '0')).profile.preferred_username, 'alice');
    assert.equal((await silentLogin(bobPage, rp, '1')).profile.preferred_username, 'bob');
    const aliceRefresh = await refresh(aliceResult);
    assert.equal((await userinfo(aliceRefresh.access_token)).sub, aliceResult.profile.sub);

    assert.equal(usedReplicas.size, 2, 'both live replicas must handle traffic');
    await proxies[0].stop();
    assert.deepEqual(await slots(context), [0, 1], 'replica switch lost the session list');
    assert.equal((await silentLogin(alicePage, rp, '0')).profile.preferred_username, 'alice');
    assert.equal((await silentLogin(bobPage, rp, '1')).profile.preferred_username, 'bob');

    await logOut(alicePage, aliceResult.tokens.id_token);
    assert.deepEqual(await slots(context), [1], 'logout did not preserve the other slot');
    assert.ok((await silentLogin(alicePage, rp, '0')).loginRequired, 'logged-out slot still has SSO');
    assert.equal((await silentLogin(bobPage, rp, '1')).profile.preferred_username, 'bob');
    const bobRefresh = await refresh(bobResult);
    assert.equal((await userinfo(bobRefresh.access_token)).sub, bobResult.profile.sub);
    const revoked = await tokenResponse({
      grant_type: 'refresh_token',
      refresh_token: aliceRefresh.refresh_token,
    });
    assert.equal(revoked.status, 400, 'logged-out account still had a valid refresh token');
    assert.equal((await revoked.json()).error, 'invalid_grant');
    assert.ok(!cookieNames.has('KEYCLOAK_IDENTITY'), 'an identity cookie reached the browser');
    console.log('Browser Set-Cookie names:', [...cookieNames].sort().join(', '));
    console.log('Keycloak OIDC passed: two sessions, account switching, refresh, replica failover, and targeted logout.');
  } finally {
    await browser?.close();
    await Promise.all(proxies.map(proxy => proxy.stop()));
    for (const server of servers) server.close();
  }
}

assert.ok(process.argv[2], 'pass the path to the IdMux binary');
await run(process.argv[2]);
