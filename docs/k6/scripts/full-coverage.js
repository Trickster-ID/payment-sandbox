/**
 * full-coverage — deterministic sweep hitting every endpoint at least once.
 * Objective: coverage completeness, not peak load.
 * Each scenario runs with its own executor and a small VU count.
 */
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { run as runPublic } from '../scenarios/public.js';
import { run as runAuth } from '../scenarios/auth.js';
import { run as runMerchant, setup as setupMerchant } from '../scenarios/merchant.js';
import { run as runAdmin, setup as setupAdmin } from '../scenarios/admin.js';
import { run as runTopup } from '../scenarios/lifecycle_topup.js';
import { run as runPayment } from '../scenarios/lifecycle_payment.js';
import { run as runRefund, setup as setupRefund } from '../scenarios/lifecycle_refund.js';
import { run as runOAuth2Client } from '../scenarios/lifecycle_oauth2_client.js';
import { makeHandleSummary } from '../helpers/report.js';
const thresholds = JSON.parse(open('../fixtures/thresholds.json'));

export let options = {
  scenarios: {
    public_check: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 3,
      exec: 'scenarioPublic',
    },
    auth_check: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 2,
      exec: 'scenarioAuth',
      startTime: '5s',
    },
    merchant_check: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 3,
      exec: 'scenarioMerchant',
      startTime: '10s',
    },
    admin_check: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 3,
      exec: 'scenarioAdmin',
      startTime: '10s',
    },
    topup_lifecycle: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 2,
      exec: 'scenarioTopup',
      startTime: '20s',
    },
    payment_lifecycle: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 2,
      exec: 'scenarioPayment',
      startTime: '20s',
    },
    refund_lifecycle: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 2,
      exec: 'scenarioRefund',
      startTime: '30s',
    },
    oauth2_client_lifecycle: {
      executor: 'per-vu-iterations',
      vus: 2,
      iterations: 2,
      exec: 'scenarioOAuth2Client',
      startTime: '20s',
    },
  },
  thresholds,
};

export function setup() {
  const adminToken = getAdminToken();
  const merchantToken = getMerchantToken();

  const merchantData = setupMerchant();
  const adminData = setupAdmin();
  const refundSetupData = setupRefund();

  return {
    adminToken,
    merchantToken,
    seedInvoiceId: merchantData.seedInvoiceId,
    seedTopupId: adminData.seedTopupId,
    seedPaymentIntentId: adminData.seedPaymentIntentId,
    seedMerchantId: adminData.seedMerchantId,
    refundAdminToken: refundSetupData.adminToken,
    refundMerchantToken: refundSetupData.merchantToken,
  };
}

export function scenarioPublic(data) { runPublic(data); }
export function scenarioAuth(_data) { runAuth({}); }
export function scenarioMerchant(data) { runMerchant(data); }
export function scenarioAdmin(data) { runAdmin(data); }
export function scenarioTopup(data) { runTopup(data); }
export function scenarioPayment(data) { runPayment(data); }
export function scenarioRefund(data) {
  runRefund({ adminToken: data.refundAdminToken, merchantToken: data.refundMerchantToken });
}
export function scenarioOAuth2Client(data) { runOAuth2Client(data); }

export function handleSummary(data) {
  return makeHandleSummary(data, 'full-coverage');
}
