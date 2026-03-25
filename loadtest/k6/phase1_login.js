import http from 'k6/http';
import { sleep } from 'k6';
import exec from 'k6/execution';

import { getBaseURL, parseEnvelope, pickBuyer } from './lib.js';

export const options = {
  scenarios: {
    login_baseline: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.LOGIN_RATE || 3),
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: Number(__ENV.PRE_VUS || 5),
      maxVUs: Number(__ENV.MAX_VUS || 10),
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<1000'],
  },
};

export default function () {
  const user = pickBuyer(exec);
  const response = http.post(
    `${getBaseURL()}/api/v1/user/login`,
    JSON.stringify({
      username: user.username,
      password: user.password,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
      },
    }
  );

  const payload = parseEnvelope(response);
  if (!payload || !payload.data || !payload.data.access_token) {
    throw new Error(`login response missing token for ${user.username}`);
  }
  sleep(Number(__ENV.SLEEP_SECONDS || 0.2));
}
