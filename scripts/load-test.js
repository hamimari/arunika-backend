// k6 load test for the pre-release gate: the anonymous read endpoints the app
// hits on launch. Run via scripts/prerelease_check.sh, or directly:
//   TARGET_URL=http://localhost:8080 k6 run scripts/load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.TARGET_URL || 'http://localhost:8080';

const PATHS = [
  '/health',
  '/ar/cards',
  '/ar/categories',
  '/fairy-tales',
  '/dongeng-categories',
  '/premium/packs',
  '/banners',
  '/app/feature-flags',
];

export const options = {
  vus: 20,
  duration: '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
};

export default function () {
  for (const path of PATHS) {
    const res = http.get(`${BASE}${path}`, { tags: { name: path } });
    check(res, { [`${path} is 200`]: (r) => r.status === 200 });
  }
  sleep(1);
}
