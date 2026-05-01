import http from 'k6/http';
import { check, fail } from 'k6';
import { BASE_URL, OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET, ADMIN_EMAIL, ADMIN_PASSWORD, MERCHANT_EMAIL, MERCHANT_PASSWORD } from './env.js';
import { registerPayload } from './data_factory.js';

function formEncode(obj) {
  return Object.entries(obj)
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
    .join('&');
}

export function tokenByPassword(email, password) {
  const params = formEncode({
    grant_type: 'password',
    username: email,
    password: password,
    client_id: OAUTH2_CLIENT_ID,
    client_secret: OAUTH2_CLIENT_SECRET,
    scope: 'read write',
  });

  const res = http.post(
    `${BASE_URL}/api/v1/oauth2/token`,
    params,
    { headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, tags: { endpoint: 'oauth2_token' } }
  );

  check(res, { 'oauth2/token: status 200': (r) => r.status === 200 });

  const body = JSON.parse(res.body);
  if (!body.data || !body.data.access_token) {
    fail(`Failed to obtain token for ${email}: ${res.body}`);
  }
  return { accessToken: body.data.access_token, refreshToken: body.data.refresh_token || '' };
}

export function getAdminToken() {
  return tokenByPassword(ADMIN_EMAIL, ADMIN_PASSWORD).accessToken;
}

export function getMerchantToken() {
  if (!MERCHANT_EMAIL || !MERCHANT_PASSWORD) return registerAndLogin();
  return tokenByPassword(MERCHANT_EMAIL, MERCHANT_PASSWORD).accessToken;
}

export function registerAndLogin() {
  const payload = registerPayload();
  const parsed = JSON.parse(payload);

  const regRes = http.post(
    `${BASE_URL}/api/v1/users/register`,
    payload,
    { headers: { 'Content-Type': 'application/json' }, tags: { endpoint: 'users_register' } }
  );

  check(regRes, { 'users/register: status 201 or 200': (r) => r.status === 201 || r.status === 200 });

  return tokenByPassword(parsed.email, parsed.password).accessToken;
}

export function bearerHeaders(token, extra) {
  return Object.assign({ Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, extra || {});
}

export function refreshToken(refreshTok) {
  const params = formEncode({
    grant_type: 'refresh_token',
    refresh_token: refreshTok,
    client_id: OAUTH2_CLIENT_ID,
    client_secret: OAUTH2_CLIENT_SECRET,
  });

  const res = http.post(
    `${BASE_URL}/api/v1/oauth2/token`,
    params,
    { headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, tags: { endpoint: 'oauth2_token_refresh' } }
  );

  check(res, { 'oauth2/token refresh: status 200': (r) => r.status === 200 });
  const body = JSON.parse(res.body);
  return body.data ? body.data.access_token : '';
}
