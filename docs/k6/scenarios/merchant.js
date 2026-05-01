import { sleep } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { get, post, del } from '../helpers/client.js';
import { checkOK, checkCreated, checkStatus, parseData } from '../helpers/checks.js';
import { idempotencyHeaders } from '../helpers/idempotency.js';
import { getMerchantToken } from '../helpers/auth.js';
import { createInvoicePayload, createTopupPayload, createClientPayload } from '../helpers/data_factory.js';

export function setup() {
  const token = getMerchantToken();

  // Pre-create one invoice so GET /merchant/invoices has data
  const iRes = post(
    token,
    `${BASE_URL}/api/v1/merchant/invoices`,
    createInvoicePayload(),
    'merchant_invoices_post',
    idempotencyHeaders()
  );
  const invoice = parseData(iRes);

  return {
    merchantToken: token,
    seedInvoiceId: invoice ? invoice.id : null,
  };
}

export function run(data) {
  const token = data.merchantToken;
  const url = (path) => `${BASE_URL}/api/v1${path}`;

  // GET /merchant/wallet
  checkOK(get(token, url('/merchant/wallet'), 'merchant_wallet_get'), 'GET /merchant/wallet');
  sleep(0.2);

  // GET /merchant/topups
  checkOK(get(token, `${url('/merchant/topups')}?page=1&limit=10`, 'merchant_topups_list'), 'GET /merchant/topups');
  sleep(0.2);

  // POST /merchant/topups (each VU creates its own)
  const topupRes = post(token, url('/merchant/topups'), createTopupPayload(100000), 'merchant_topups_post', idempotencyHeaders());
  checkCreated(topupRes, 'POST /merchant/topups');
  sleep(0.2);

  // POST /merchant/invoices
  const invRes = post(token, url('/merchant/invoices'), createInvoicePayload(), 'merchant_invoices_post', idempotencyHeaders());
  checkCreated(invRes, 'POST /merchant/invoices');
  const invoice = parseData(invRes);
  sleep(0.2);

  // GET /merchant/invoices
  checkOK(get(token, `${url('/merchant/invoices')}?page=1&limit=10`, 'merchant_invoices_list'), 'GET /merchant/invoices');
  sleep(0.2);

  // GET /merchant/invoices/:id
  const invId = (invoice && invoice.id) || data.seedInvoiceId;
  if (invId) {
    checkOK(get(token, `${url('/merchant/invoices')}/${invId}`, 'merchant_invoices_get'), 'GET /merchant/invoices/:id');
    sleep(0.2);
  }

  // POST /merchant/refunds is skipped here — invoice is never paid in this scenario,
  // so it always returns 400 and would inflate http_req_failed. The full flow
  // (request → review → process) is covered by lifecycle_refund.

  // GET /merchant/refunds
  checkOK(get(token, `${url('/merchant/refunds')}?page=1&limit=10`, 'merchant_refunds_list'), 'GET /merchant/refunds');
  sleep(0.2);

  // POST /merchant/clients
  const clientRes = post(token, url('/merchant/clients'), createClientPayload(), 'merchant_clients_post');
  checkCreated(clientRes, 'POST /merchant/clients');
  const client = parseData(clientRes);
  const clientId = client ? client.client.id : null;
  sleep(0.2);

  // GET /merchant/clients
  checkOK(get(token, url('/merchant/clients'), 'merchant_clients_list'), 'GET /merchant/clients');
  sleep(0.2);

  // DELETE /merchant/clients/:id
  if (clientId) {
    const delRes = del(token, `${url('/merchant/clients')}/${clientId}`, 'merchant_clients_delete');
    checkStatus(delRes, 'DELETE /merchant/clients/:id', 200);
    sleep(0.2);
  }

  sleep(0.5);
}

export default function () {
  const data = setup();
  run(data);
}
