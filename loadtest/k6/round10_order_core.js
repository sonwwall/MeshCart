import http from 'k6/http';
import exec from 'k6/execution';
import { sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

import { bizTagsOf, getBaseURL, getManifest, parseEnvelope } from './lib.js';

const manifest = getManifest();
const hotProduct = manifest.hot_product;
const normalProducts = manifest.normal_products || [];

export const orderCreateDuration = new Trend('order_create_duration', true);
export const orderCreateFailed = new Rate('order_create_failed');
export const orderCreateFailedTotal = new Counter('order_create_failed_total');

export const options = {
  vus: Number(__ENV.VUS || 20),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    order_create_failed: ['rate<0.05'],
    order_create_duration: ['p(95)<1500'],
  },
};

function extractInt64(body, field) {
  const matched = body.match(new RegExp(`"${field}"\\s*:\\s*(\\d+)`));
  return matched ? matched[1] : '';
}

function login(username, password) {
  const response = http.post(
    `${getBaseURL()}/api/v1/user/login`,
    JSON.stringify({ username, password }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: 'setup_login_for_round10_order' },
      timeout: __ENV.LOGIN_TIMEOUT || '5s',
    }
  );
  const payload = parseEnvelope(response);
  if (!payload || !payload.data || !payload.data.access_token) {
    throw new Error(`setup login failed for ${username}`);
  }
  return payload.data.access_token;
}

function resolveTarget() {
  const mode = (__ENV.PRODUCT_POOL || 'hot').toLowerCase();
  if (mode === 'normal') {
    if (normalProducts.length === 0) {
      throw new Error('normal product pool is empty');
    }
    const idx = exec.scenario.iterationInTest % normalProducts.length;
    const product = normalProducts[idx];
    return {
      product_id: product.product_id,
      sku_id: product.sku_id,
      mode,
    };
  }

  if (!hotProduct) {
    throw new Error('hot product is missing');
  }
  return {
    product_id: hotProduct.product_id,
    sku_id: hotProduct.sku_id,
    mode,
  };
}

export function setup() {
  const buyer = manifest.buyers && manifest.buyers.length > 0 ? manifest.buyers[0] : null;
  if (!buyer) {
    throw new Error('no buyer found in manifest');
  }
  return {
    token: login(buyer.username, buyer.password),
  };
}

export default function (data) {
  const target = resolveTarget();
  const requestID = `round10-order-${target.mode}-${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${Date.now()}`;
  const response = http.post(
    `${getBaseURL()}/api/v1/orders`,
    `{"request_id":"${requestID}","items":[{"product_id":${target.product_id},"sku_id":${target.sku_id},"quantity":${Number(__ENV.QUANTITY || 1)}}]}`,
    {
      headers: {
        'Content-Type': 'application/json',
        Authorization: data.token,
      },
      tags: {
        name: 'round10_create_order',
        product_pool: target.mode,
      },
      timeout: __ENV.ORDER_TIMEOUT || '5s',
    }
  );

  orderCreateDuration.add(response.timings.duration, { product_pool: target.mode });
  const payload = parseEnvelope(response);
  const orderID = extractInt64(response.body, 'order_id');
  const ok = !!(payload && payload.code === 0 && orderID);
  orderCreateFailed.add(!ok, { product_pool: target.mode });
  if (!ok) {
    const tags = bizTagsOf(payload, 'unknown');
    orderCreateFailedTotal.add(1, {
      product_pool: target.mode,
      code: tags.code,
      reason: tags.reason,
    });
    throw new Error(`create order failed in ${target.mode} mode code=${tags.code} reason=${tags.reason}`);
  }

  sleep(Number(__ENV.SLEEP_SECONDS || 0.1));
}
