/**
 * baseline — moderate load, steady VUs.
 * Primary non-functional benchmark. Run this before and after significant changes.
 */
import { sleep } from 'k6';
import { VUS, DURATION, RAMP_UP } from '../helpers/env.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { run as runMerchant } from '../scenarios/merchant.js';
import { run as runAdmin } from '../scenarios/admin.js';
import { makeHandleSummary } from '../helpers/report.js';
const thresholds = JSON.parse(open('../fixtures/thresholds.json'));

export let options = {
  stages: [
    { duration: RAMP_UP, target: VUS },
    { duration: DURATION, target: VUS },
    { duration: '10s', target: 0 },
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
  // Alternate between merchant and admin flows based on VU ID parity
  if (__VU % 2 === 0) {
    runMerchant(data);
  } else {
    runAdmin(data);
  }
  sleep(0.5);
}

export function handleSummary(data) {
  return makeHandleSummary(data, 'baseline');
}
