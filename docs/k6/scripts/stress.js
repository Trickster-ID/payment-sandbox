/**
 * stress — ramp above expected load to find degradation points.
 */
import { sleep } from 'k6';
import { VUS } from '../helpers/env.js';
import { getAdminToken, getMerchantToken } from '../helpers/auth.js';
import { run as runMerchant } from '../scenarios/merchant.js';
import { run as runAdmin } from '../scenarios/admin.js';
import { makeHandleSummary } from '../helpers/report.js';

const peakVUs = VUS * 3;

export let options = {
  stages: [
    { duration: '30s', target: VUS },
    { duration: '1m', target: peakVUs },
    { duration: '1m', target: peakVUs },
    { duration: '30s', target: VUS },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
    'http_req_duration': ['p(95)<1000'],
    checks: ['rate>0.95'],
  },
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
  sleep(0.3);
}

export function handleSummary(data) {
  return makeHandleSummary(data, 'stress');
}
