import http from 'k6/http';
import { sleep } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { checkOK, checkCreated, parseData } from '../helpers/checks.js';

export function setup() {
  return {};
}

export function run(data) {
  // GET /api/v1/ping
  const pingRes = http.get(`${BASE_URL}/api/v1/ping`, { tags: { endpoint: 'ping' } });
  checkOK(pingRes, 'ping');

  sleep(0.3);

  // GET /api/v1/pay/:token — use token from data if available
  if (data && data.invoiceToken) {
    const payRes = http.get(
      `${BASE_URL}/api/v1/pay/${data.invoiceToken}`,
      { tags: { endpoint: 'pay_token_get' } }
    );
    checkOK(payRes, 'GET /pay/:token');

    sleep(0.3);

    // POST /api/v1/pay/:token/intents
    const intentRes = http.post(
      `${BASE_URL}/api/v1/pay/${data.invoiceToken}/intents`,
      JSON.stringify({ method: 'WALLET' }),
      { headers: { 'Content-Type': 'application/json' }, tags: { endpoint: 'pay_token_intents' } }
    );
    // 201 on first create, 409/400 if already has intent — both are acceptable for load test
    const ok = intentRes.status === 201 || intentRes.status === 409 || intentRes.status === 400;
    if (!ok) {
      checkCreated(intentRes, 'POST /pay/:token/intents');
    }

    sleep(0.3);
  }
}

export default function () {
  run({});
}
