/**
 * soak — sustained run at moderate load to detect memory leaks and gradual degradation.
 * Default: 10 VUs for 30 minutes. Override with K6_VUS and K6_DURATION.
 */
import { sleep } from 'k6';
import { VUS, DURATION } from '../helpers/env.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { run as runMerchant } from '../scenarios/merchant.js';
import { run as runAdmin } from '../scenarios/admin.js';
import { makeHandleSummary } from '../helpers/report.js';
const thresholds = JSON.parse(open('../fixtures/thresholds.json'));

export let options = {
  stages: [
    { duration: '2m', target: VUS },
    { duration: DURATION || '30m', target: VUS },
    { duration: '2m', target: 0 },
  ],
  thresholds,
};

export function setup() {
  return {
    merchantToken: getMerchantToken(),
    adminToken: getAdminToken(),
  };
}

export default function (data) {
  if (__VU % 2 === 0) {
    runMerchant(data);
  } else {
    runAdmin(data);
  }
  sleep(1);
}

export function handleSummary(data) {
  return makeHandleSummary(data, 'soak');
}
