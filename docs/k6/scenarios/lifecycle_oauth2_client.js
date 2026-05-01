/**
 * Lifecycle: oauth2 client
 * Merchant registers client → lists clients → deletes client.
 */
import { sleep } from 'k6';
import { check } from 'k6';
import { BASE_URL } from '../helpers/env.js';
import { get, post as clientPost, del } from '../helpers/client.js';
import { checkCreated, checkOK, checkStatus, parseData } from '../helpers/checks.js';
import { getMerchantToken } from '../helpers/auth.js';
import { createClientPayload } from '../helpers/data_factory.js';

export function setup() {
  return { merchantToken: getMerchantToken() };
}

export function run(data) {
  const url = (path) => `${BASE_URL}/api/v1${path}`;
  const { merchantToken } = data;

  // Step 1: Register new oauth2 client
  const createRes = clientPost(
    merchantToken,
    url('/merchant/clients'),
    createClientPayload(),
    'lifecycle_oauth2_client_create'
  );
  checkCreated(createRes, 'lifecycle oauth2 client: POST /merchant/clients');
  const created = parseData(createRes);
  const clientId = created && created.client ? created.client.id : null;
  sleep(0.3);

  // Step 2: List clients — verify new client appears
  const listRes = get(merchantToken, url('/merchant/clients'), 'lifecycle_oauth2_client_list');
  checkOK(listRes, 'lifecycle oauth2 client: GET /merchant/clients');
  const clients = parseData(listRes);
  check(null, {
    'lifecycle oauth2 client: created client in list': () =>
      Array.isArray(clients) && clients.some((c) => c.id === clientId),
  });
  sleep(0.3);

  // Step 3: Delete the client
  if (clientId) {
    const delRes = del(merchantToken, `${url('/merchant/clients')}/${clientId}`, 'lifecycle_oauth2_client_delete');
    checkStatus(delRes, 'lifecycle oauth2 client: DELETE /merchant/clients/:id', 200);
    sleep(0.3);

    // Step 4: Verify it no longer appears
    const listAfter = parseData(get(merchantToken, url('/merchant/clients'), 'lifecycle_oauth2_client_list_after'));
    check(null, {
      'lifecycle oauth2 client: deleted client absent': () =>
        !Array.isArray(listAfter) || !listAfter.some((c) => c.id === clientId),
    });
  }

  sleep(0.5);
}

export default function () {
  const data = setup();
  run(data);
}
