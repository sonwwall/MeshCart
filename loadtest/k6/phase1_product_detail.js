import http from 'k6/http';
import { sleep } from 'k6';

import { getBaseURL, getManifest, parseEnvelope } from './lib.js';

const manifest = getManifest();
const defaultProductID = manifest.hot_product && manifest.hot_product.product_id;

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<1000'],
  },
};

export default function () {
  const productID = __ENV.PRODUCT_ID || defaultProductID;
  if (!productID) {
    throw new Error('product_id is required');
  }
  const response = http.get(`${getBaseURL()}/api/v1/products/detail/${productID}`);
  const payload = parseEnvelope(response);
  if (!payload || !payload.data || !payload.data.id) {
    throw new Error('product detail response missing product');
  }
  sleep(Number(__ENV.SLEEP_SECONDS || 0.2));
}
