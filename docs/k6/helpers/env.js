import { fail } from 'k6';

function required(name) {
  const v = __ENV[name];
  if (!v || v.trim() === '') fail(`Missing required env var: ${name}`);
  return v.trim();
}

function optional(name, defaultValue) {
  const v = __ENV[name];
  return v && v.trim() !== '' ? v.trim() : defaultValue;
}

export const BASE_URL = required('K6_BASE_URL').replace(/\/$/, '');
export const ADMIN_EMAIL = required('K6_ADMIN_EMAIL');
export const ADMIN_PASSWORD = required('K6_ADMIN_PASSWORD');
export const OAUTH2_CLIENT_ID = required('K6_OAUTH2_CLIENT_ID');
export const OAUTH2_CLIENT_SECRET = required('K6_OAUTH2_CLIENT_SECRET');

export const MERCHANT_EMAIL = optional('K6_MERCHANT_EMAIL', '');
export const MERCHANT_PASSWORD = optional('K6_MERCHANT_PASSWORD', '');

export const VUS = parseInt(optional('K6_VUS', '5'), 10);
export const DURATION = optional('K6_DURATION', '30s');
export const RAMP_UP = optional('K6_RAMP_UP', '10s');
export const REPORT_DIR = optional('K6_REPORT_DIR', 'docs/k6/reports');
