import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET } from '../helpers/env.js';

import { checkOK, checkCreated } from '../helpers/checks.js';
import { registerPayload } from '../helpers/data_factory.js';
import { tokenByPassword, bearerHeaders } from '../helpers/auth.js';

export function setup() {
  return {};
}

export function run(_data) {
  // POST /api/v1/users/register
  const payload = registerPayload();
  const parsed = JSON.parse(payload);
  const regRes = http.post(
    `${BASE_URL}/api/v1/users/register`,
    payload,
    { headers: { 'Content-Type': 'application/json' }, tags: { endpoint: 'users_register' } }
  );
  checkCreated(regRes, 'POST /users/register');

  sleep(0.3);

  // POST /api/v1/oauth2/token — password grant
  const tokenData = tokenByPassword(parsed.email, parsed.password);
  const accessToken = tokenData.accessToken;
  const refreshTok = tokenData.refreshToken;

  sleep(0.3);

  // POST /api/v1/oauth2/introspect
  const introspectBody = `token=${encodeURIComponent(accessToken)}`;
  const introRes = http.post(
    `${BASE_URL}/api/v1/oauth2/introspect`,
    introspectBody,
    { headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, tags: { endpoint: 'oauth2_introspect' } }
  );
  check(introRes, {
    'oauth2/introspect: status 200': (r) => r.status === 200,
    'oauth2/introspect: active': (r) => {
      try { return JSON.parse(r.body).data.active === true; } catch (_) { return false; }
    },
  });

  sleep(0.3);

  // GET /api/v1/oauth2/userinfo (authenticated)
  const userInfoRes = http.get(
    `${BASE_URL}/api/v1/oauth2/userinfo`,
    { headers: bearerHeaders(accessToken), tags: { endpoint: 'oauth2_userinfo' } }
  );
  checkOK(userInfoRes, 'GET /oauth2/userinfo');

  sleep(0.3);

  // POST /api/v1/oauth2/token — refresh_token grant (if refresh token available)
  if (refreshTok) {
    const refreshParams = [
      `grant_type=refresh_token`,
      `refresh_token=${encodeURIComponent(refreshTok)}`,
      `client_id=${encodeURIComponent(OAUTH2_CLIENT_ID)}`,
      `client_secret=${encodeURIComponent(OAUTH2_CLIENT_SECRET)}`,
    ].join('&');
    const refreshRes = http.post(
      `${BASE_URL}/api/v1/oauth2/token`,
      refreshParams,
      { headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, tags: { endpoint: 'oauth2_token_refresh' } }
    );
    check(refreshRes, { 'oauth2/token refresh: status 200': (r) => r.status === 200 });

    sleep(0.3);
  }

  // POST /api/v1/oauth2/revoke
  const revokeParams = [
    `token=${encodeURIComponent(accessToken)}`,
    `client_id=${encodeURIComponent(OAUTH2_CLIENT_ID)}`,
    `client_secret=${encodeURIComponent(OAUTH2_CLIENT_SECRET)}`,
  ].join('&');
  const revokeRes = http.post(
    `${BASE_URL}/api/v1/oauth2/revoke`,
    revokeParams,
    { headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, tags: { endpoint: 'oauth2_revoke' } }
  );
  check(revokeRes, { 'oauth2/revoke: status 200': (r) => r.status === 200 });

  sleep(0.5);

  // GET /api/v1/oauth2/authorize — browser-based endpoint; 4xx without a session is expected.
  // responseCallback prevents 4xx from inflating http_req_failed.
  const authUrl = `${BASE_URL}/api/v1/oauth2/authorize?response_type=code&client_id=${OAUTH2_CLIENT_ID}&redirect_uri=http%3A%2F%2Flocalhost%3A3000%2Fcb&state=perftest`;
  const authorizeRes = http.get(authUrl, {
    tags: { endpoint: 'oauth2_authorize_get' },
    responseCallback: http.expectedStatuses({ min: 200, max: 499 }),
  });
  check(authorizeRes, { 'oauth2/authorize GET: responds': (r) => r.status < 500 });

  sleep(0.3);
}

export default function () {
  run({});
}
