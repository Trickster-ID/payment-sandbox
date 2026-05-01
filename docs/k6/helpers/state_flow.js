import { parseData } from './checks.js';

export function extractId(res) {
  const d = parseData(res);
  return d ? d.id : null;
}

export function extractField(res, field) {
  const d = parseData(res);
  return d ? d[field] : null;
}

export function extractFirstId(res) {
  const d = parseData(res);
  if (!d || !Array.isArray(d) || d.length === 0) return null;
  return d[0].id;
}

export function assertId(id, label) {
  if (!id) throw new Error(`state_flow: no id returned for ${label}`);
  return id;
}
