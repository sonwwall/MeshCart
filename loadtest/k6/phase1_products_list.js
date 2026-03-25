import http from 'k6/http';
import { sleep } from 'k6';

import { getBaseURL, parseEnvelope } from './lib.js';

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<1000'],
  },
};

export default function () {
  const response = http.get(`${getBaseURL()}/api/v1/products?page=1&page_size=10`);
  const payload = parseEnvelope(response);
  if (!payload || !payload.data || !Array.isArray(payload.data.products)) {
    throw new Error('products list response missing products');
  }
  sleep(Number(__ENV.SLEEP_SECONDS || 0.2));
}
