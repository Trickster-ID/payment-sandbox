/**
 * smoke — quick health check: 1 VU, short duration.
 * Validates env, auth, and essential endpoint reachability.
 * Safe to run in CI on every push.
 */
import http from 'k6/http';
import { sleep } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { checkOK } from '../helpers/checks.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { makeHandleSummary } from '../helpers/report.js';

export let options = {
  vus: 1,
  duration: '20s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{endpoint:ping}': ['p(95)<100'],
    'http_req_duration{endpoint:oauth2_token}': ['p(95)<500'],
    checks: ['rate>0.99'],
  },
};

export function setup() {
  return {
    merchantToken: getMerchantToken(),
    adminToken: getAdminToken(),
  };
}

export default function (data) {
  const url = (path) => `${BASE_URL}/api/v1${path}`;

  checkOK(http.get(url('/ping'), { tags: { endpoint: 'ping' } }), 'smoke: ping');
  sleep(0.5);

  checkOK(
    http.get(url('/merchant/wallet'), { headers: { Authorization: `Bearer ${data.merchantToken}`, 'Content-Type': 'application/json' }, tags: { endpoint: 'merchant_wallet_get' } }),
    'smoke: merchant wallet'
  );
  sleep(0.5);

  checkOK(
    http.get(`${url('/admin/topups')}?page=1&limit=5`, { headers: { Authorization: `Bearer ${data.adminToken}`, 'Content-Type': 'application/json' }, tags: { endpoint: 'admin_topups_list' } }),
    'smoke: admin topups'
  );
  sleep(0.5);
}

export function handleSummary(data) {
  return makeHandleSummary(data, 'smoke');
}
