/**
 * Lifecycle: refund
 * Full paid-invoice flow → merchant requests refund → admin reviews → admin processes.
 */
import { sleep } from 'k6';
import { check } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { get, post as clientPost, patch, publicPost } from '../helpers/client.js';
import { checkCreated, checkOK, parseData } from '../helpers/checks.js';
import { idempotencyHeaders } from '../helpers/idempotency.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { createInvoicePayload, createRefundPayload } from '../helpers/data_factory.js';

export function setup() {
  const adminToken = getAdminToken();
  const merchantToken = getMerchantToken();

  // Pre-fund merchant wallet so refund can be processed
  const url = (path) => `${BASE_URL}/api/v1${path}`;
  const topupRes = clientPost(
    merchantToken,
    url('/merchant/topups'),
    JSON.stringify({ amount: 500000 }),
    'refund_setup_topup',
    idempotencyHeaders()
  );
  const topup = parseData(topupRes);
  if (topup && topup.id) {
    patch(adminToken, `${url('/admin/topups')}/${topup.id}/status`, JSON.stringify({ status: 'SUCCESS' }), 'refund_setup_topup_approve');
  }

  return { adminToken, merchantToken };
}

export function run(data) {
  const url = (path) => `${BASE_URL}/api/v1${path}`;
  const { adminToken, merchantToken } = data;

  // Step 1: Merchant creates invoice
  const invRes = clientPost(
    merchantToken,
    url('/merchant/invoices'),
    createInvoicePayload(60000),
    'lifecycle_refund_invoice_create',
    idempotencyHeaders()
  );
  checkCreated(invRes, 'lifecycle refund: POST /merchant/invoices');
  const invoice = parseData(invRes);
  if (!invoice || !invoice.payment_link_token) return;
  const payToken = invoice.payment_link_token;
  const invoiceId = invoice.id;
  sleep(0.3);

  // Step 2: Customer pays
  const intentRes = publicPost(
    url(`/pay/${payToken}/intents`),
    JSON.stringify({ method: 'WALLET' }),
    'lifecycle_refund_pay_intent'
  );
  checkCreated(intentRes, 'lifecycle refund: POST /pay/:token/intents');
  const intentData = parseData(intentRes);
  if (!intentData || !intentData.payment_intent) return;
  const intentId = intentData.payment_intent.id;
  sleep(0.3);

  // Step 3: Admin marks payment SUCCESS
  checkOK(
    patch(adminToken, `${url('/admin/payment-intents')}/${intentId}/status`, JSON.stringify({ status: 'SUCCESS' }), 'lifecycle_refund_pay_approve'),
    'lifecycle refund: PATCH /admin/payment-intents/:id/status'
  );
  sleep(0.3);

  // Step 4: Merchant requests refund
  const refundRes = clientPost(
    merchantToken,
    url('/merchant/refunds'),
    createRefundPayload(invoiceId),
    'lifecycle_refund_request',
    idempotencyHeaders()
  );
  checkCreated(refundRes, 'lifecycle refund: POST /merchant/refunds');
  const refund = parseData(refundRes);
  if (!refund || !refund.id) return;
  const refundId = refund.id;
  sleep(0.3);

  // Step 5: Merchant sees refund in their list
  checkOK(
    get(merchantToken, `${url('/merchant/refunds')}?page=1&limit=10`, 'lifecycle_refund_list_merchant'),
    'lifecycle refund: GET /merchant/refunds'
  );
  sleep(0.3);

  // Step 6: Admin reviews refund (APPROVE)
  checkOK(
    patch(adminToken, `${url('/admin/refunds')}/${refundId}/review`, JSON.stringify({ decision: 'APPROVE' }), 'lifecycle_refund_review'),
    'lifecycle refund: PATCH /admin/refunds/:id/review'
  );
  sleep(0.3);

  // Step 7: Admin processes refund
  const processRes = patch(
    adminToken,
    `${url('/admin/refunds')}/${refundId}/process`,
    JSON.stringify({ status: 'SUCCESS' }),
    'lifecycle_refund_process'
  );
  checkOK(processRes, 'lifecycle refund: PATCH /admin/refunds/:id/process');
  sleep(0.3);

  // Step 8: Verify final refund status
  // ProcessRefund returns { refund: {...}, merchant: {...} }
  const refundData = parseData(processRes);
  check(null, {
    'lifecycle refund: status is SUCCESS': () =>
      refundData && refundData.refund && refundData.refund.status === 'SUCCESS',
  });

  sleep(0.5);
}

export default function () {
  const data = setup();
  run(data);
}
