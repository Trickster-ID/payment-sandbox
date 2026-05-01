import { check } from 'k6';

export function checkOK(res, tag) {
  return check(res, {
    [`${tag}: status 200`]: (r) => r.status === 200,
    [`${tag}: has data`]: (r) => {
      try { return JSON.parse(r.body).data !== undefined; } catch (_) { return false; }
    },
  });
}

export function checkCreated(res, tag) {
  return check(res, {
    [`${tag}: status 201`]: (r) => r.status === 201,
    [`${tag}: has data`]: (r) => {
      try { return JSON.parse(r.body).data !== undefined; } catch (_) { return false; }
    },
  });
}

export function checkNoError(res, tag) {
  return check(res, {
    [`${tag}: no error`]: (r) => {
      try { return JSON.parse(r.body).error === undefined; } catch (_) { return false; }
    },
  });
}

export function checkStatus(res, tag, status) {
  return check(res, {
    [`${tag}: status ${status}`]: (r) => r.status === status,
  });
}

export function parseData(res) {
  try { return JSON.parse(res.body).data; } catch (_) { return null; }
}
