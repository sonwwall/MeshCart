import http from 'k6/http';
import exec from 'k6/execution';
import { sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

import { getBaseURL, getManifest, parseEnvelope, pickBuyer } from './lib.js';

const manifest = getManifest();
const hotProduct = manifest.hot_product;

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
  const orderBody = `{"request_id":"${requestID}","items":[{"product_id":${hotProduct.product_id},"sku_id":${hotProduct.sku_id},"quantity":1}]}`;
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
