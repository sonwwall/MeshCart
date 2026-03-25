import http from 'k6/http';
import exec from 'k6/execution';
import { sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

import { bizTagsOf, getBaseURL, getManifest, parseEnvelope } from './lib.js';

const manifest = getManifest();
const hotProduct = manifest.hot_product;

export const checkoutDuration = new Trend('checkout_duration', true);
export const orderCreateDuration = new Trend('checkout_order_create_duration', true);
export const paymentCreateDuration = new Trend('checkout_payment_create_duration', true);
export const paymentConfirmDuration = new Trend('checkout_payment_confirm_duration', true);
export const checkoutFailed = new Rate('checkout_failed');
export const checkoutFailedTotal = new Counter('checkout_failed_total');

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checkout_failed: ['rate<0.05'],
    checkout_duration: ['p(95)<2500'],
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
      tags: { name: 'setup_login_for_round10_checkout' },
      timeout: __ENV.LOGIN_TIMEOUT || '5s',
    }
  );
  const payload = parseEnvelope(response);
  if (!payload || !payload.data || !payload.data.access_token) {
    throw new Error(`setup login failed for ${username}`);
  }
  return payload.data.access_token;
}

export function setup() {
  const buyer = manifest.buyers && manifest.buyers.length > 0 ? manifest.buyers[0] : null;
  if (!buyer) {
    throw new Error('no buyer found in manifest');
  }
  if (!hotProduct) {
    throw new Error('hot product is missing');
  }
  return {
    token: login(buyer.username, buyer.password),
  };
}

export default function (data) {
  const startedAt = Date.now();
  const suffix = `${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${startedAt}`;

  const orderResponse = http.post(
    `${getBaseURL()}/api/v1/orders`,
    `{"request_id":"round10-checkout-order-${suffix}","items":[{"product_id":${hotProduct.product_id},"sku_id":${hotProduct.sku_id},"quantity":${Number(__ENV.QUANTITY || 1)}}]}`,
    {
      headers: {
        'Content-Type': 'application/json',
        Authorization: data.token,
      },
      tags: { name: 'round10_checkout_create_order' },
      timeout: __ENV.ORDER_TIMEOUT || '5s',
    }
  );
  orderCreateDuration.add(orderResponse.timings.duration);

  const orderPayload = parseEnvelope(orderResponse);
  const orderID = extractInt64(orderResponse.body, 'order_id');
  if (!(orderPayload && orderPayload.code === 0 && orderID)) {
    const tags = bizTagsOf(orderPayload, 'unknown');
    checkoutFailed.add(true, { stage: 'create_order' });
    checkoutFailedTotal.add(1, { stage: 'create_order', code: tags.code, reason: tags.reason });
    throw new Error(`checkout create order failed code=${tags.code} reason=${tags.reason}`);
  }

  const paymentResponse = http.post(
    `${getBaseURL()}/api/v1/payments`,
    `{"order_id":${orderID},"payment_method":"mock","request_id":"round10-checkout-pay-${suffix}"}`,
    {
      headers: {
        'Content-Type': 'application/json',
        Authorization: data.token,
      },
      tags: { name: 'round10_checkout_create_payment' },
      timeout: __ENV.PAYMENT_TIMEOUT || '5s',
    }
  );
  paymentCreateDuration.add(paymentResponse.timings.duration);

  const paymentPayload = parseEnvelope(paymentResponse);
  const paymentID = extractInt64(paymentResponse.body, 'payment_id');
  if (!(paymentPayload && paymentPayload.code === 0 && paymentID)) {
    const tags = bizTagsOf(paymentPayload, 'unknown');
    checkoutFailed.add(true, { stage: 'create_payment' });
    checkoutFailedTotal.add(1, { stage: 'create_payment', code: tags.code, reason: tags.reason });
    throw new Error(`checkout create payment failed code=${tags.code} reason=${tags.reason}`);
  }

  const confirmResponse = http.post(
    `${getBaseURL()}/api/v1/payments/${paymentID}/mock_success`,
    `{"request_id":"round10-checkout-confirm-${suffix}","payment_trade_no":"round10-trade-${suffix}"}`,
    {
      headers: {
        'Content-Type': 'application/json',
        Authorization: data.token,
      },
      tags: { name: 'round10_checkout_confirm_payment' },
      timeout: __ENV.PAYMENT_CONFIRM_TIMEOUT || '5s',
    }
  );
  paymentConfirmDuration.add(confirmResponse.timings.duration);

  const confirmPayload = parseEnvelope(confirmResponse);
  const ok = !!(confirmPayload && confirmPayload.code === 0 && extractInt64(confirmResponse.body, 'payment_id'));
  checkoutFailed.add(!ok, { stage: 'confirm_payment' });
  checkoutDuration.add(Date.now() - startedAt);
  if (!ok) {
    const tags = bizTagsOf(confirmPayload, 'unknown');
    checkoutFailedTotal.add(1, { stage: 'confirm_payment', code: tags.code, reason: tags.reason });
    throw new Error(`checkout confirm payment failed code=${tags.code} reason=${tags.reason}`);
  }

  sleep(Number(__ENV.SLEEP_SECONDS || 0.1));
}
