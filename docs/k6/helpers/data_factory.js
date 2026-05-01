import { uuidv4 } from './idempotency.js';

export function randomEmail() {
  return `perf_${uuidv4().slice(0, 8)}@sandbox.test`;
}

export function randomName() {
  const names = ['Alice', 'Bob', 'Carol', 'Dave', 'Eve', 'Frank'];
  return names[Math.floor(Math.random() * names.length)] + ' Perf';
}

export function registerPayload() {
  return JSON.stringify({
    name: randomName(),
    email: randomEmail(),
    password: 'Perf1234!',
  });
}

export function createInvoicePayload(amount) {
  const due = new Date(Date.now() + 7 * 24 * 3600 * 1000).toISOString();
  return JSON.stringify({
    customer_name: randomName(),
    customer_email: randomEmail(),
    amount: amount || 50000,
    description: 'perf test invoice',
    due_date: due,
  });
}

export function createTopupPayload(amount) {
  return JSON.stringify({ amount: amount || 100000 });
}

export function createRefundPayload(invoiceId) {
  return JSON.stringify({
    invoice_id: invoiceId,
    reason: 'perf test refund reason',
  });
}

export function createClientPayload() {
  return JSON.stringify({
    name: `perf-client-${uuidv4().slice(0, 6)}`,
    redirect_uris: ['http://localhost:9999/callback'],
    scopes: ['read'],
  });
}

export function createPaymentIntentPayload() {
  return JSON.stringify({
    customer_name: randomName(),
    customer_email: randomEmail(),
    payment_method: 'BANK_TRANSFER',
  });
}
