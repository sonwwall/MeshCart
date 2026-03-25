import { check } from 'k6';

const manifestPath = __ENV.MANIFEST || '../results/phase1-manifest.json';
const manifest = JSON.parse(open(manifestPath));

export function getManifest() {
  return manifest;
}

export function getBaseURL() {
  return (__ENV.BASE_URL || manifest.base_url || 'http://127.0.0.1:8080').replace(/\/$/, '');
}

export function parseEnvelope(response) {
  let payload = null;
  try {
    payload = response.json();
  } catch (_err) {
    payload = null;
  }

  check(response, {
    'http status is 200': (r) => r.status === 200,
    'response body is json': () => payload !== null,
    'business code is 0': () => payload && payload.code === 0,
  });

  return payload;
}

export function bizCodeOf(payload) {
  if (!payload || typeof payload.code !== 'number') {
    return 'unknown';
  }
  return String(payload.code);
}

export function bizReasonOf(payload, fallback = 'unknown') {
  if (!payload || typeof payload.code !== 'number') {
    return fallback;
  }

  switch (payload.code) {
    case 0:
      return 'ok';
    case 1000001:
      return 'invalid_param';
    case 1000002:
      return 'unauthorized';
    case 1000003:
      return 'forbidden';
    case 1000004:
      return 'not_found';
    case 1000005:
      return 'rate_limited';
    case 1009999:
      return payload.message && payload.message.includes('下游服务暂不可用')
        ? 'service_unavailable'
        : 'internal_error';
    case 2040001:
      return 'order_not_found';
    case 2040002:
      return 'invalid_order_data';
    case 2040003:
      return 'product_unavailable';
    case 2040004:
      return 'sku_unavailable';
    case 2040005:
      return 'insufficient_stock';
    case 2040006:
      return 'state_conflict';
    case 2040007:
      return 'paid_immutable';
    case 2040008:
      return 'payment_conflict';
    case 2040009:
      return 'idempotency_busy';
    default:
      return `code_${payload.code}`;
  }
}

export function bizTagsOf(payload, fallback = 'unknown') {
  return {
    code: bizCodeOf(payload),
    reason: bizReasonOf(payload, fallback),
  };
}

export function pickBuyer(exec) {
  const buyers = manifest.buyers || [];
  if (buyers.length === 0) {
    throw new Error('no buyers found in manifest');
  }
  const idx = exec.vu.idInTest % buyers.length;
  return buyers[idx];
}
