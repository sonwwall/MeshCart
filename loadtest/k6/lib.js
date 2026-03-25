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

export function pickBuyer(exec) {
  const buyers = manifest.buyers || [];
  if (buyers.length === 0) {
    throw new Error('no buyers found in manifest');
  }
  const idx = exec.vu.idInTest % buyers.length;
  return buyers[idx];
}
