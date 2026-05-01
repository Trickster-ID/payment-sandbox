import { sleep } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { get, post as clientPost, publicPost } from '../helpers/client.js';
import { checkOK, parseData } from '../helpers/checks.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { idempotencyHeaders } from '../helpers/idempotency.js';
import { createTopupPayload, createInvoicePayload } from '../helpers/data_factory.js';
import { extractId } from '../helpers/state_flow.js';

export function setup() {
  const adminToken = getAdminToken();
  const merchantToken = getMerchantToken();
  const url = (path) => `${BASE_URL}/api/v1${path}`;

  // Create a topup for admin to act on
  const topupRes = clientPost(merchantToken, url('/merchant/topups'), createTopupPayload(50000), 'setup_topup', idempotencyHeaders());
  const topupId = extractId(topupRes);

  // Create an invoice for payment intent
  const invRes = clientPost(merchantToken, url('/merchant/invoices'), createInvoicePayload(), 'setup_invoice', idempotencyHeaders());
  const invoice = parseData(invRes);
  const invoiceToken = invoice ? invoice.payment_link_token : null;

  // Create a payment intent (public endpoint — no auth)
  let paymentIntentId = null;
  if (invoiceToken) {
    const piRes = publicPost(url(`/pay/${invoiceToken}/intents`), JSON.stringify({ method: 'WALLET' }), 'setup_payment_intent');
    const pi = parseData(piRes);
    paymentIntentId = pi ? pi.payment_intent.id : null;
  }

  // Get merchant ID from wallet
  const walletRes = get(merchantToken, url('/merchant/wallet'), 'setup_wallet');
  const wallet = parseData(walletRes);
  const merchantId = wallet ? wallet.id : null;

  return {
    adminToken,
    merchantToken,
    seedTopupId: topupId,
    seedPaymentIntentId: paymentIntentId,
    seedMerchantId: merchantId,
  };
}

export function run(data) {
  const token = data.adminToken;
  const url = (path) => `${BASE_URL}/api/v1${path}`;

  // GET /admin/topups
  checkOK(get(token, `${url('/admin/topups')}?page=1&limit=10`, 'admin_topups_list'), 'GET /admin/topups');
  sleep(0.2);

  // GET /admin/payment-intents
  checkOK(get(token, `${url('/admin/payment-intents')}?page=1&limit=10`, 'admin_payment_intents_list'), 'GET /admin/payment-intents');
  sleep(0.2);

  // GET /admin/refunds
  checkOK(get(token, `${url('/admin/refunds')}?page=1&limit=10`, 'admin_refunds_list'), 'GET /admin/refunds');
  sleep(0.2);

  // GET /admin/stats
  checkOK(get(token, url('/admin/stats'), 'admin_stats'), 'GET /admin/stats');
  sleep(0.2);

  // GET /admin/ledger/accounts/:merchant_id
  if (data.seedMerchantId) {
    checkOK(
      get(token, `${url('/admin/ledger/accounts')}/${data.seedMerchantId}`, 'admin_ledger_account'),
      'GET /admin/ledger/accounts/:merchant_id'
    );
    sleep(0.2);
  }

  sleep(0.5);
}

export default function () {
  const data = setup();
  run(data);
}
