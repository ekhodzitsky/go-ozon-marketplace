import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 50 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
  },
};

const GRAPHQL_URL = __ENV.GRAPHQL_URL || 'http://localhost:8080/query';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

export default function () {
  const payload = JSON.stringify({
    query: `
      mutation {
        createProduct(
          name: "Benchmark Product",
          description: "Created by k6 benchmark",
          price: 99.99,
          categories: ["benchmark", "test"]
        )
      }
    `,
  });

  const headers = {
    'Content-Type': 'application/json',
  };

  if (AUTH_TOKEN) {
    headers['Authorization'] = `Bearer ${AUTH_TOKEN}`;
  }

  const res = http.post(GRAPHQL_URL, payload, { headers });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'no errors': (r) => {
      const body = r.json();
      return body.errors === undefined || body.errors === null;
    },
  });

  sleep(1);
}
