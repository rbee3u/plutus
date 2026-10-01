#!/usr/bin/env bash
set -euo pipefail

for file in client-ca.pem client.pem client-key.pem; do
  if [[ -e "$file" || -L "$file" ]]; then
    printf 'Refusing to replace existing certificate file: %s\n' "$file" >&2
    exit 1
  fi
done

umask 077
cert_dir=$(mktemp -d ./.plutus-certs.XXXXXX)
trap 'rm -rf -- "$cert_dir"' EXIT

(
  cd "$cert_dir"

  openssl genrsa -out client-ca-key.pem 2048
  openssl req -x509 -new -days 1825 \
    -key client-ca-key.pem \
    -out client-ca.pem \
    -subj "/CN=client-ca.plutus-prod.self" \
    -addext "basicConstraints = critical, CA:TRUE" \
    -addext "keyUsage = critical, keyCertSign, cRLSign"

  openssl genrsa -out client-key.pem 2048
  openssl req -new \
    -key client-key.pem \
    -out client.csr \
    -subj "/CN=client.plutus-prod.self"
  cat <<EOF >client.ext
basicConstraints = CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF
  openssl x509 -req -days 1825 \
    -in client.csr \
    -CA client-ca.pem \
    -CAkey client-ca-key.pem \
    -CAcreateserial \
    -out client.pem \
    -extfile client.ext

  openssl verify -purpose sslclient -CAfile client-ca.pem client.pem

  chmod 644 client-ca.pem
  chmod 644 client.pem
  chmod 600 client-key.pem
)

mv "$cert_dir/client-ca.pem" "$cert_dir/client.pem" "$cert_dir/client-key.pem" .

cat <<EOF
Generated Cloudflare origin pull mTLS certificates:
  client-ca.pem
  client.pem
  client-key.pem

Upload to Cloudflare Authenticated Origin Pulls:
  client.pem
  client-key.pem

Suggested prod.toml value:
[server.tls]
cert_file = "server.pem"
key_file = "server-key.pem"
client_ca_file = "client-ca.pem"

Use Cloudflare Origin CA for server.pem and server-key.pem
EOF
