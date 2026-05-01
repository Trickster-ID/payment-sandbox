/**
 * Lifecycle: topup
 * Merchant creates topup → admin approves → merchant wallet balance reflects credit.
 */
import { sleep } from 'k6';
import { check } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { get, post as clientPost, patch } from '../helpers/client.js';
import { checkCreated, checkOK, parseData } from '../helpers/checks.js';
import { idempotencyHeaders } from '../helpers/idempotency.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { createTopupPayload } from '../helpers/data_factory.js';

export function setup() {
  return {
    adminToken: getAdminToken(),
    merchantToken: getMerchantToken(),
  };
}

export function run(data) {
  const url = (path) => `${BASE_URL}/api/v1${path}`;
  const { adminToken, merchantToken } = data;

  // Step 1: Merchant gets current wallet balance
  const walletBefore = parseData(get(merchantToken, url('/merchant/wallet'), 'lifecycle_wallet_before'));
  const balanceBefore = walletBefore ? walletBefore.balance : 0;
  sleep(0.3);

  // Step 2: Merchant creates topup
  const topupAmount = 100000;
  const topupRes = clientPost(
    merchantToken,
    url('/merchant/topups'),
    createTopupPayload(topupAmount),
    'lifecycle_topup_create',
    idempotencyHeaders()
  );
  checkCreated(topupRes, 'lifecycle: POST /merchant/topups');
  const topup = parseData(topupRes);
  if (!topup || !topup.id) return;
  const topupId = topup.id;
  sleep(0.3);

  // Step 3: Merchant sees topup in their list
  const listRes = get(merchantToken, `${url('/merchant/topups')}?page=1&limit=10`, 'lifecycle_topup_list');
  checkOK(listRes, 'lifecycle: GET /merchant/topups');
  sleep(0.3);

  // Step 4: Admin approves the topup
  const approveRes = patch(
    adminToken,
    `${url('/admin/topups')}/${topupId}/status`,
    JSON.stringify({ status: 'SUCCESS' }),
    'lifecycle_topup_approve'
  );
  checkOK(approveRes, 'lifecycle: PATCH /admin/topups/:id/status');
  sleep(0.3);

  // Step 5: Merchant wallet balance increased
  const walletAfter = parseData(get(merchantToken, url('/merchant/wallet'), 'lifecycle_wallet_after'));
  const balanceAfter = walletAfter ? walletAfter.balance : 0;
  check(null, {
    'lifecycle topup: balance increased': () => balanceAfter >= balanceBefore + topupAmount,
  });

  sleep(0.5);
}

export default function () {
  const data = setup();
  run(data);
}
