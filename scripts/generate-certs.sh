#!/usr/bin/env bash
set -euo pipefail

# Generates CA, server and client TLS certificates for localhost gRPC.
# Usage: ./scripts/generate-certs.sh [output_dir]
# Default output dir: certs

OUT_DIR="${1:-${CERT_PATH:-certs}}"
DAYS=365
KEY_BITS=4096

echo "Generating TLS certificates in: $OUT_DIR"
mkdir -p "$OUT_DIR"

# --- CA ---
openssl genrsa -out "$OUT_DIR/ca-key.pem" $KEY_BITS
openssl req -new -x509 -days $DAYS -key "$OUT_DIR/ca-key.pem" -out "$OUT_DIR/ca-cert.pem" \
  -subj "/CN=localhost-ca/O=go-ozon-marketplace"

# --- Server ---
openssl genrsa -out "$OUT_DIR/server-key.pem" $KEY_BITS
openssl req -new -key "$OUT_DIR/server-key.pem" -out "$OUT_DIR/server.csr" \
  -subj "/CN=localhost/O=go-ozon-marketplace"

# SAN config for localhost / 127.0.0.1
cat > "$OUT_DIR/server-ext.cnf" <<EOF
subjectAltName = DNS:localhost, IP:127.0.0.1
EOF

openssl x509 -req -days $DAYS -in "$OUT_DIR/server.csr" \
  -CA "$OUT_DIR/ca-cert.pem" -CAkey "$OUT_DIR/ca-key.pem" -CAcreateserial \
  -out "$OUT_DIR/server-cert.pem" -extfile "$OUT_DIR/server-ext.cnf"

# --- Client ---
openssl genrsa -out "$OUT_DIR/client-key.pem" $KEY_BITS
openssl req -new -key "$OUT_DIR/client-key.pem" -out "$OUT_DIR/client.csr" \
  -subj "/CN=client/O=go-ozon-marketplace"

openssl x509 -req -days $DAYS -in "$OUT_DIR/client.csr" \
  -CA "$OUT_DIR/ca-cert.pem" -CAkey "$OUT_DIR/ca-key.pem" -CAcreateserial \
  -out "$OUT_DIR/client-cert.pem"

# Cleanup intermediates
rm -f "$OUT_DIR/server.csr" "$OUT_DIR/client.csr" "$OUT_DIR/server-ext.cnf" "$OUT_DIR/ca-cert.srl"

echo "Done. Files in $OUT_DIR:"
ls -la "$OUT_DIR"
