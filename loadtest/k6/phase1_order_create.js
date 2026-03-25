import http from 'k6/http';
import exec from 'k6/execution';
import { sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

import { getBaseURL, getManifest, parseEnvelope, pickBuyer } from './lib.js';

const manifest = getManifest();
const defaultProduct = manifest.hot_product;
const normalProduct = manifest.normal_products && manifest.normal_products.length > 0 ? manifest.normal_products[0] : null;

export const orderCreateDuration = new Trend('order_create_duration', true);
export const orderCreateFailed = new Rate('order_create_failed');

export const options = {
  vus: Number(__ENV.VUS || 2),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    order_create_failed: ['rate<0.01'],
    order_create_duration: ['p(95)<1500'],
  },
};

function resolveTargetProduct() {
  const productID = __ENV.PRODUCT_ID;
  const skuID = __ENV.SKU_ID;
  if (productID && skuID) {
    return { product_id: productID, sku_id: skuID };
  }
  if (__ENV.USE_NORMAL_PRODUCT === '1') {
    if (!normalProduct) {
      throw new Error('normal product not found in manifest');
    }
    return {
      product_id: normalProduct.product_id,
      sku_id: normalProduct.sku_id,
    };
  }
  return {
    product_id: defaultProduct.product_id,
    sku_id: defaultProduct.sku_id,
  };
}

function extractInt64(body, field) {
  const matched = body.match(new RegExp(`"${field}"\\s*:\\s*(\\d+)`));
  return matched ? matched[1] : '';
}

function login(user) {
  const response = http.post(
    `${getBaseURL()}/api/v1/user/login`,
    JSON.stringify({
      username: user.username,
      password: user.password,
    }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: 'login_for_order' },
    }
  );
  const payload = parseEnvelope(response);
  if (!payload || !payload.data || !payload.data.access_token) {
    throw new Error(`login failed for ${user.username}`);
  }
  return payload.data.access_token;
}

export default function () {
  const user = pickBuyer(exec);
  const token = login(user);
  const requestID = `order-${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${Date.now()}`;
  const target = resolveTargetProduct();
  const orderBody = `{"request_id":"${requestID}","items":[{"product_id":${target.product_id},"sku_id":${target.sku_id},"quantity":1}]}`;
  const response = http.post(
    `${getBaseURL()}/api/v1/orders`,
    orderBody,
    {
      headers: {
        'Content-Type': 'application/json',
        Authorization: token,
      },
      tags: { name: 'create_order' },
    }
  );

  orderCreateDuration.add(response.timings.duration);
  const payload = parseEnvelope(response);
  const orderID = extractInt64(response.body, 'order_id');
  const ok = !!(payload && payload.code === 0 && orderID);
  orderCreateFailed.add(!ok);
  if (!ok) {
    throw new Error('create order response missing order_id');
  }
  sleep(Number(__ENV.SLEEP_SECONDS || 0.5));
}
