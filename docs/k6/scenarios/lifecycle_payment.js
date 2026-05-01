/**
 * Lifecycle: payment
 * Merchant creates invoice → public customer views invoice and creates payment intent →
 * admin approves payment → invoice becomes PAID.
 */
import { sleep } from 'k6';
import { check } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { get, post as clientPost, patch, publicPost, publicGet } from '../helpers/client.js';
import { checkCreated, checkOK, parseData } from '../helpers/checks.js';
import { idempotencyHeaders } from '../helpers/idempotency.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { createInvoicePayload } from '../helpers/data_factory.js';

export function setup() {
  return {
    adminToken: getAdminToken(),
    merchantToken: getMerchantToken(),
  };
}

export function run(data) {
  const url = (path) => `${BASE_URL}/api/v1${path}`;
  const { adminToken, merchantToken } = data;

  // Step 1: Merchant creates invoice
  const invRes = clientPost(
    merchantToken,
    url('/merchant/invoices'),
    createInvoicePayload(75000),
    'lifecycle_invoice_create',
    idempotencyHeaders()
  );
  checkCreated(invRes, 'lifecycle: POST /merchant/invoices');
  const invoice = parseData(invRes);
  if (!invoice || !invoice.payment_link_token) return;
  const token = invoice.payment_link_token;
  const invoiceId = invoice.id;
  sleep(0.3);

  // Step 2: Public customer views invoice
  const pubRes = publicGet(url(`/pay/${token}`), 'lifecycle_pay_get');
  checkOK(pubRes, 'lifecycle: GET /pay/:token');
  sleep(0.3);

  // Step 3: Public customer creates payment intent
  const intentRes = publicPost(
    url(`/pay/${token}/intents`),
    JSON.stringify({ method: 'WALLET' }),
    'lifecycle_pay_intent'
  );
  checkCreated(intentRes, 'lifecycle: POST /pay/:token/intents');
  const intentData = parseData(intentRes);
  if (!intentData || !intentData.payment_intent) return;
  const intentId = intentData.payment_intent.id;
  sleep(0.3);

  // Step 4: Admin lists payment intents (verify it appears)
  checkOK(
    get(adminToken, `${url('/admin/payment-intents')}?page=1&limit=20`, 'lifecycle_admin_pi_list'),
    'lifecycle: GET /admin/payment-intents'
  );
  sleep(0.3);

  // Step 5: Admin marks payment as SUCCESS
  const approveRes = patch(
    adminToken,
    `${url('/admin/payment-intents')}/${intentId}/status`,
    JSON.stringify({ status: 'SUCCESS' }),
    'lifecycle_admin_pi_approve'
  );
  checkOK(approveRes, 'lifecycle: PATCH /admin/payment-intents/:id/status');
  sleep(0.3);

  // Step 6: Merchant sees invoice is now PAID
  const invDetail = parseData(get(merchantToken, `${url('/merchant/invoices')}/${invoiceId}`, 'lifecycle_invoice_detail'));
  check(null, {
    'lifecycle payment: invoice is PAID': () => invDetail && invDetail.status === 'PAID',
  });

  sleep(0.5);
}

export default function () {
  const data = setup();
  run(data);
}
