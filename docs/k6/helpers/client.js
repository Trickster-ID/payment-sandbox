import http from 'k6/http';
import { bearerHeaders } from './auth.js';

export function get(token, path, tag) {
  return http.get(
    path,
    { headers: bearerHeaders(token), tags: { endpoint: tag || path } }
  );
}

export function post(token, path, body, tag, extraHeaders) {
  return http.post(
    path,
    body,
    { headers: Object.assign(bearerHeaders(token), extraHeaders || {}), tags: { endpoint: tag || path } }
  );
}

export function patch(token, path, body, tag) {
  return http.patch(
    path,
    body,
    { headers: bearerHeaders(token), tags: { endpoint: tag || path } }
  );
}

export function del(token, path, tag) {
  return http.del(
    path,
    null,
    { headers: bearerHeaders(token), tags: { endpoint: tag || path } }
  );
}

export function publicGet(path, tag) {
  return http.get(path, { tags: { endpoint: tag || path } });
}

export function publicPost(path, body, tag) {
  return http.post(
    path,
    body,
    { headers: { 'Content-Type': 'application/json' }, tags: { endpoint: tag || path } }
  );
}
