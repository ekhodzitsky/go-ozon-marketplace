import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
  },
};

const GRAPHQL_URL = __ENV.GRAPHQL_URL || 'http://localhost:8080/query';

export default function () {
  const payload = JSON.stringify({
    query: `
      query {
        searchProducts(query: "phone", page: 1, pageSize: 20) {
          products {
            id
            name
            price
          }
          total
        }
      }
    `,
  });

  const headers = {
    'Content-Type': 'application/json',
  };

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
